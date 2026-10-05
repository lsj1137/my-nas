package main

import (
	"sync"
	"time"
)

const (
	maxFailures  = 5
	lockDuration = 15 * time.Minute
)

type failState struct {
	count       int
	lockedUntil time.Time
}

// Lockout은 사용자별 연속 로그인 실패를 세고 일정 횟수가 넘으면 잠근다.
type Lockout struct {
	mu    sync.Mutex
	state map[string]*failState
	now   func() time.Time
}

func NewLockout() *Lockout {
	return &Lockout{state: make(map[string]*failState), now: time.Now}
}

func (l *Lockout) Locked(user string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[user]
	return ok && l.now().Before(st.lockedUntil)
}

func (l *Lockout) Fail(user string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[user]
	if !ok {
		st = &failState{}
		l.state[user] = st
	}
	st.count++
	if st.count >= maxFailures {
		st.lockedUntil = l.now().Add(lockDuration)
		st.count = 0
	}
}

func (l *Lockout) Success(user string) {
	l.mu.Lock()
	delete(l.state, user)
	l.mu.Unlock()
}
