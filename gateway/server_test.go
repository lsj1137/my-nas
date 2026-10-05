package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const testOrigin = "https://nas.example.com"

func testConfig(t *testing.T) *Config {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("owner-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	yml := `
public_origin: ` + testOrigin + `
ip_map:
  - cidr: 192.168.0.0/24
    user: shared
  - cidr: 203.0.113.10
    user: owner
users:
  owner:
    password_hash: "` + string(hash) + `"
  shared:
    login: false
`
	if err := os.WriteFile(p, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// authReq는 Nginx auth_request 서브요청을 흉내 낸다. bound는 브라우저에 연결된 사용자 쿠키 값("" = 없음).
func authReq(ip string, cookie *http.Cookie, bound string) *http.Request {
	r := httptest.NewRequest("GET", "/_auth", nil)
	r.Header.Set("X-Real-IP", ip)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if bound != "" {
		r.AddCookie(&http.Cookie{Name: boundCookieName, Value: bound})
	}
	return r
}

func login(t *testing.T, h http.Handler, ip, user, pw string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"username": {user}, "password": {pw}}
	r := httptest.NewRequest("POST", "/_gw/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", testOrigin)
	r.Header.Set("X-Real-IP", ip)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func sessionCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestUserForIP(t *testing.T) {
	cfg := testConfig(t)
	cases := map[string]string{
		"192.168.0.7":        "shared",
		"203.0.113.10":       "owner",
		"203.0.113.11":       "",
		"::ffff:192.168.0.9": "shared",
		"garbage":            "",
		"":                   "",
	}
	for ip, want := range cases {
		got, _ := cfg.UserForIP(ip)
		if got != want {
			t.Errorf("UserForIP(%q) = %q, want %q", ip, got, want)
		}
	}
}

func TestConfigRejectsUnknownIPMapUser(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(p, []byte("public_origin: https://a.b\nusers: {a: {login: false}}\nip_map: [{cidr: 10.0.0.0/8, user: nobody}]\n"), 0o600)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for unknown ip_map user")
	}
}

func TestAuthFlow(t *testing.T) {
	srv := NewServer(testConfig(t), "")
	h := srv.Handler()

	// 매핑 없는 IP → 401
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authReq("198.51.100.1", nil, ""))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unmapped ip: got %d", w.Code)
	}

	// 매핑된 IP인데 아직 연결 기록 없음 → 403 (reset으로 연결)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("192.168.0.5", nil, ""))
	if w.Code != http.StatusForbidden {
		t.Fatalf("unbound: got %d", w.Code)
	}

	// 매핑된 IP + shared로 연결됨 → 200 + shared
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("192.168.0.5", nil, "shared"))
	if w.Code != 200 || w.Header().Get("X-Nas-User") != "shared" {
		t.Fatalf("mapped ip: got %d %q", w.Code, w.Header().Get("X-Nas-User"))
	}

	// 로그인 → 세션 사용자가 IP 매핑보다 우선
	lw := login(t, h, "192.168.0.5", "owner", "owner-password")
	c := sessionCookie(lw)
	if c == nil || lw.Header().Get("Location") != "/_gw/reset" {
		t.Fatalf("login failed: %d %v", lw.Code, lw.Header())
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie attributes: %+v", c)
	}

	// 로그인 직후 브라우저는 아직 shared로 연결된 상태 → 403 (Quantum 쿠키를 갈아끼워야 함)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("192.168.0.5", c, "shared"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("stale binding after login: got %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("192.168.0.5", c, "owner"))
	if w.Code != 200 || w.Header().Get("X-Nas-User") != "owner" {
		t.Fatalf("session should win: %d %q", w.Code, w.Header().Get("X-Nas-User"))
	}

	// 세션은 IP와 무관하게 동작
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("198.51.100.1", c, "owner"))
	if w.Header().Get("X-Nas-User") != "owner" {
		t.Fatalf("session from other ip: %d", w.Code)
	}

	// 로그아웃 → 세션 폐기, IP 매핑으로 복귀
	r := httptest.NewRequest("POST", "/_gw/logout", nil)
	r.Header.Set("Origin", testOrigin)
	r.AddCookie(c)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authReq("192.168.0.5", c, "shared"))
	if w.Header().Get("X-Nas-User") != "shared" {
		t.Fatalf("after logout: %q", w.Header().Get("X-Nas-User"))
	}
}

func TestResetRebindsAndExpiresQuantumCookie(t *testing.T) {
	h := NewServer(testConfig(t), "").Handler()

	r := httptest.NewRequest("GET", "/_gw/reset", nil)
	r.Header.Set("X-Real-IP", "192.168.0.5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("reset: %d %q", w.Code, w.Header().Get("Location"))
	}
	var bound string
	quantumExpired := 0
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case boundCookieName:
			bound = c.Value
		case quantumCookieName:
			if c.MaxAge < 0 {
				quantumExpired++
			}
		}
	}
	if bound != "shared" {
		t.Fatalf("bound cookie = %q", bound)
	}
	if quantumExpired != 2 {
		t.Fatalf("quantum cookie should be expired with and without Domain, got %d", quantumExpired)
	}

	// 매핑도 세션도 없으면 로그인 페이지로
	r = httptest.NewRequest("GET", "/_gw/reset", nil)
	r.Header.Set("X-Real-IP", "198.51.100.1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("Location") != "/_gw/login" {
		t.Fatalf("unmapped reset: %q", w.Header().Get("Location"))
	}
}

// 브라우저는 Referrer-Policy가 no-referrer면 폼 POST의 Origin을 "null"로 보낸다 → 로그인 불가.
func TestReferrerPolicyKeepsOrigin(t *testing.T) {
	h := NewServer(testConfig(t), "").Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/_gw/login", nil))
	if p := w.Header().Get("Referrer-Policy"); p == "no-referrer" || p == "" {
		t.Fatalf("Referrer-Policy %q would make browsers send Origin: null", p)
	}
}

func TestExampleConfigLoads(t *testing.T) {
	if _, err := LoadConfig("config.example.yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRejectsBadUsernames(t *testing.T) {
	for _, name := range []string{"admin", "../x", "a/b", "Owner", ""} {
		p := filepath.Join(t.TempDir(), "c.yaml")
		os.WriteFile(p, []byte("public_origin: https://a.b\nusers: {\""+name+"\": {login: false}}\n"), 0o600)
		if _, err := LoadConfig(p); err == nil {
			t.Errorf("username %q should be rejected", name)
		}
	}
}

func TestLoginRejections(t *testing.T) {
	srv := NewServer(testConfig(t), "")
	h := srv.Handler()

	if c := sessionCookie(login(t, h, "1.1.1.1", "owner", "wrong")); c != nil {
		t.Fatal("wrong password must not log in")
	}
	if c := sessionCookie(login(t, h, "1.1.1.1", "shared", "")); c != nil {
		t.Fatal("ip-only account must not log in")
	}
	if c := sessionCookie(login(t, h, "1.1.1.1", "nobody", "x")); c != nil {
		t.Fatal("unknown user must not log in")
	}

	// Origin 불일치 → 403
	form := url.Values{"username": {"owner"}, "password": {"owner-password"}}
	r := httptest.NewRequest("POST", "/_gw/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login: got %d", w.Code)
	}
}

func TestLockout(t *testing.T) {
	srv := NewServer(testConfig(t), "")
	h := srv.Handler()
	for i := 0; i < maxFailures; i++ {
		login(t, h, "1.1.1.1", "owner", "wrong")
	}
	if c := sessionCookie(login(t, h, "1.1.1.1", "owner", "owner-password")); c != nil {
		t.Fatal("locked account must not log in even with correct password")
	}
	srv.lockout.now = func() time.Time { return time.Now().Add(lockDuration + time.Second) }
	if c := sessionCookie(login(t, h, "1.1.1.1", "owner", "owner-password")); c == nil {
		t.Fatal("lock should expire")
	}
}

func TestSessionExpiry(t *testing.T) {
	s := NewSessionStore(time.Hour, 24*time.Hour)
	base := time.Now()
	s.now = func() time.Time { return base }
	id := s.Create("owner")

	s.now = func() time.Time { return base.Add(50 * time.Minute) }
	if _, ok := s.Lookup(id); !ok {
		t.Fatal("should be valid within idle timeout")
	}
	s.now = func() time.Time { return base.Add(2 * time.Hour) }
	if _, ok := s.Lookup(id); ok {
		t.Fatal("should expire after idle timeout")
	}

	id = s.Create("owner")
	for h := 1; h <= 25; h++ {
		s.now = func() time.Time { return base.Add(2*time.Hour + time.Duration(h)*time.Hour - time.Minute) }
		_, ok := s.Lookup(id)
		if h >= 25 && ok {
			t.Fatal("should expire after absolute timeout")
		}
	}
}

func TestReloadDropsChangedUsers(t *testing.T) {
	cfg := testConfig(t)
	srv := NewServer(cfg, "")
	id := srv.sessions.Create("owner")

	next := *cfg
	next.Users = map[string]UserConf{"owner": {PasswordHash: "$2a$04$changed"}, "shared": cfg.Users["shared"]}
	srv.Reload(&next)
	if _, ok := srv.sessions.Lookup(id); ok {
		t.Fatal("password change must drop sessions")
	}
}

func TestStaticPathTraversal(t *testing.T) {
	h := NewServer(testConfig(t), "").Handler()
	for _, p := range []string{"/_gw/static/../login.html", "/_gw/static/%2e%2e/login.html"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code == 200 && strings.Contains(w.Body.String(), "<form") {
			t.Fatalf("%s served a non-static file", p)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/_gw/static/gw.css", nil))
	if w.Code != 200 {
		t.Fatalf("static file: %d", w.Code)
	}
}
