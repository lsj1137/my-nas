package main

import (
	"net/http"
	"net/url"
)

// 브라우저가 지금 어떤 사용자로 File Browser Quantum에 붙어 있는지 기록하는 쿠키.
// Quantum은 자체 세션 쿠키가 있으면 X-Username 헤더보다 그 쿠키를 우선하고, 토큰에 사용자 이름도 없다.
// 그래서 게이트웨이가 판단한 사용자와 이 쿠키가 다르면 /_gw/reset에서 Quantum 쿠키를 만료시키고 다시 연결한다.
const (
	boundCookieName   = "__Host-nas_bound"
	quantumCookieName = "filebrowser_quantum_jwt"
)

func boundUser(r *http.Request) string {
	if c, err := r.Cookie(boundCookieName); err == nil {
		return c.Value
	}
	return ""
}

// handleReset은 Quantum 세션 쿠키를 지우고 현재 사용자로 다시 연결한 뒤 /로 보낸다.
// Quantum 쿠키는 HttpOnly라서 브라우저 스크립트로는 지울 수 없고 서버 응답으로만 만료시킬 수 있다.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	user, _ := s.resolveUser(r)
	if user == "" {
		http.Redirect(w, r, "/_gw/login", http.StatusSeeOther)
		return
	}
	expireQuantumCookie(w, s.config().PublicOrigin)
	http.SetCookie(w, &http.Cookie{
		Name:     boundCookieName,
		Value:    user,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Quantum은 Domain=<요청 호스트>로 쿠키를 굽는다. Domain 지정 여부가 다르면 다른 쿠키로 취급되므로 둘 다 만료시킨다.
func expireQuantumCookie(w http.ResponseWriter, publicOrigin string) {
	http.SetCookie(w, &http.Cookie{Name: quantumCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true})
	if u, err := url.Parse(publicOrigin); err == nil && u.Hostname() != "" {
		http.SetCookie(w, &http.Cookie{Name: quantumCookieName, Value: "", Path: "/", Domain: u.Hostname(), MaxAge: -1, HttpOnly: true, Secure: true})
	}
}

func clearBoundCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: boundCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
