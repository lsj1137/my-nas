package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "__Host-nas_session"

//go:embed web
var embeddedWeb embed.FS

// 존재하지 않는 사용자에 대해서도 bcrypt 비교를 수행해 응답 시간으로 계정 존재 여부가 드러나지 않게 한다.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)

type Server struct {
	mu        sync.RWMutex
	cfg       *Config
	sessions  *SessionStore
	lockout   *Lockout
	customDir string
	web       fs.FS
}

func NewServer(cfg *Config, customDir string) *Server {
	web, _ := fs.Sub(embeddedWeb, "web")
	return &Server{
		cfg:       cfg,
		sessions:  NewSessionStore(cfg.Session.IdleTimeout, cfg.Session.AbsoluteTimeout),
		lockout:   NewLockout(),
		customDir: customDir,
		web:       web,
	}
}

func (s *Server) config() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Reload는 새 설정을 적용한다. 삭제되었거나 로그인이 막혔거나 비밀번호가 바뀐 사용자의 세션은 폐기한다.
func (s *Server) Reload(next *Config) {
	s.mu.Lock()
	prev := s.cfg
	s.cfg = next
	s.mu.Unlock()

	s.sessions.SetTimeouts(next.Session.IdleTimeout, next.Session.AbsoluteTimeout)
	s.sessions.Retain(func(user string) bool {
		n, ok := next.Users[user]
		if !ok || !n.CanLogin() {
			return false
		}
		return prev.Users[user].PasswordHash == n.PasswordHash
	})
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_auth", s.handleAuth)
	mux.HandleFunc("GET /_gw/login", s.page("login.html"))
	mux.HandleFunc("POST /_gw/login", s.handleLogin)
	mux.HandleFunc("GET /_gw/logout", s.page("logout.html"))
	mux.HandleFunc("POST /_gw/logout", s.handleLogout)
	mux.HandleFunc("POST /_gw/logout-all", s.handleLogoutAll)
	mux.HandleFunc("GET /_gw/whoami", s.handleWhoami)
	mux.HandleFunc("GET /_gw/reset", s.handleReset)
	mux.HandleFunc("GET /_gw/vault", s.requireUser(s.page("vault.html")))
	mux.HandleFunc("GET /_gw/static/", s.handleStatic)
	return securityHeaders(mux)
}

// resolveUser는 세션 쿠키 → IP 매핑 순서로 사용자를 판단한다.
func (s *Server) resolveUser(r *http.Request) (user, via string) {
	cfg := s.config()
	if c, err := r.Cookie(cookieName); err == nil {
		if u, ok := s.sessions.Lookup(c.Value); ok {
			if _, exists := cfg.Users[u]; exists {
				return u, "session"
			}
		}
	}
	if u, ok := cfg.UserForIP(clientIP(r)); ok {
		return u, "ip"
	}
	return "", ""
}

// clientIP는 Nginx가 $remote_addr로 설정한 X-Real-IP만 신뢰한다. 게이트웨이는 127.0.0.1에서만 수신해야 한다.
func clientIP(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Real-IP"))
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	user, _ := s.resolveUser(r)
	if user == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// 브라우저에 연결된 Quantum 세션이 다른 사용자 것이거나 연결 기록이 없으면 차단 → Nginx가 /_gw/reset으로 보낸다.
	if boundUser(r) != user {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("X-Nas-User", user)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.PostForm.Get("username"))
	password := r.PostForm.Get("password")
	ip := clientIP(r)

	cfg := s.config()
	uc, exists := cfg.Users[username]
	hash := dummyHash
	if exists && uc.CanLogin() {
		hash = []byte(uc.PasswordHash)
	}
	pwOK := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	locked := exists && s.lockout.Locked(username)

	if !exists || !uc.CanLogin() || locked || !pwOK {
		if exists && !locked {
			s.lockout.Fail(username)
		}
		log.Printf("login failed user=%q ip=%s locked=%t", username, ip, locked)
		http.Redirect(w, r, "/_gw/login?error=1", http.StatusSeeOther)
		return
	}

	s.lockout.Success(username)
	if c, err := r.Cookie(cookieName); err == nil {
		s.sessions.Delete(c.Value)
	}
	id := s.sessions.Create(username)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   int(cfg.Session.AbsoluteTimeout.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	log.Printf("login ok user=%q ip=%s", username, ip)
	// 이전 계정의 File Browser 토큰을 지우고 들어가도록 reset을 거친다.
	http.Redirect(w, r, "/_gw/reset", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if c, err := r.Cookie(cookieName); err == nil {
		s.sessions.Delete(c.Value)
	}
	s.dropBrowserState(w)
	http.Redirect(w, r, "/_gw/reset", http.StatusSeeOther)
}

// dropBrowserState는 게이트웨이 세션 쿠키와 Quantum 연결 상태를 모두 지운다.
// 이후 /_gw/reset이 IP 매핑 사용자로 다시 연결하거나, 매핑이 없으면 로그인 페이지로 보낸다.
func (s *Server) dropBrowserState(w http.ResponseWriter) {
	clearCookie(w)
	clearBoundCookie(w)
	expireQuantumCookie(w, s.config().PublicOrigin)
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	user, via := s.resolveUser(r)
	if via != "session" {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	s.sessions.DeleteUser(user)
	s.dropBrowserState(w)
	log.Printf("logout-all user=%q ip=%s", user, clientIP(r))
	http.Redirect(w, r, "/_gw/reset", http.StatusSeeOther)
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	user, via := s.resolveUser(r)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if user == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"user": nil})
		return
	}
	// bound가 false면 브라우저의 Quantum 세션이 현재 사용자와 어긋난 상태 → overlay.js가 /_gw/reset으로 보낸다.
	json.NewEncoder(w).Encode(map[string]any{"user": user, "via": via, "bound": boundUser(r) == user})
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user, _ := s.resolveUser(r); user == "" {
			http.Redirect(w, r, "/_gw/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == s.config().PublicOrigin
}

func clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// readWebFile은 custom 디렉터리에 같은 이름의 파일이 있으면 그것을, 없으면 내장 파일을 돌려준다.
func (s *Server) readWebFile(name string) ([]byte, error) {
	if s.customDir != "" {
		if b, err := os.ReadFile(filepath.Join(s.customDir, filepath.FromSlash(name))); err == nil {
			return b, nil
		}
	}
	return fs.ReadFile(s.web, name)
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := s.readWebFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(b)
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/_gw/"))
	if !strings.HasPrefix(name, "static/") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	b, err := s.readWebFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(b)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		// no-referrer로 두면 브라우저가 폼 POST의 Origin을 "null"로 보내서 sameOrigin 검사에 정상 로그인까지 걸린다.
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
