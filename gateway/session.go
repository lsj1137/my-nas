package main

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type session struct {
	user     string
	created  time.Time
	lastSeen time.Time
}

// SessionStore는 메모리 세션 저장소. 재시작하면 모든 로그인 세션이 사라진다.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*session
	idle     time.Duration
	absolute time.Duration
	now      func() time.Time
}

func NewSessionStore(idle, absolute time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*session),
		idle:     idle,
		absolute: absolute,
		now:      time.Now,
	}
}

func newSessionID() string {
	b := make([]byte, 32) // 256비트
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *SessionStore) Create(user string) string {
	id := newSessionID()
	now := s.now()
	s.mu.Lock()
	s.sessions[id] = &session{user: user, created: now, lastSeen: now}
	s.mu.Unlock()
	return id
}

// Lookup은 유효한 세션의 사용자를 돌려주고 마지막 사용 시각을 갱신한다.
func (s *SessionStore) Lookup(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return "", false
	}
	if now.Sub(sess.lastSeen) > s.idle || now.Sub(sess.created) > s.absolute {
		delete(s.sessions, id)
		return "", false
	}
	sess.lastSeen = now
	return sess.user, true
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *SessionStore) DeleteUser(user string) {
	s.mu.Lock()
	for id, sess := range s.sessions {
		if sess.user == user {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

// Retain은 keep이 false를 돌려주는 사용자의 세션을 모두 삭제한다 (설정 재적용 시 사용).
func (s *SessionStore) Retain(keep func(user string) bool) {
	s.mu.Lock()
	for id, sess := range s.sessions {
		if !keep(sess.user) {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

func (s *SessionStore) SetTimeouts(idle, absolute time.Duration) {
	s.mu.Lock()
	s.idle, s.absolute = idle, absolute
	s.mu.Unlock()
}

func (s *SessionStore) Sweep() {
	now := s.now()
	s.mu.Lock()
	for id, sess := range s.sessions {
		if now.Sub(sess.lastSeen) > s.idle || now.Sub(sess.created) > s.absolute {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}
