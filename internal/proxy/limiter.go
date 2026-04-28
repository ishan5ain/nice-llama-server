package proxy

import (
	"sync"
	"time"
)

type rateLimiter struct {
	now   func() time.Time
	mu    sync.Mutex
	users map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		now:   time.Now,
		users: make(map[string][]time.Time),
	}
}

func (l *rateLimiter) Allow(user string, rpm int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	windowStart := now.Add(-time.Minute)
	timestamps := l.users[user]
	kept := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= rpm {
		l.users[user] = kept
		return false
	}
	l.users[user] = append(kept, now)
	return true
}
