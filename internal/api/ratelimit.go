package api

import (
	"sync"
	"time"
)

// failureLimiter refuses an endpoint after too many failed credential checks
// in a sliding window. One limiter guards the whole install rather than one
// per client address: behind a reverse proxy every request carries the proxy's
// address anyway, and a household has no legitimate use for dozens of failed
// passwords a minute, whoever they come from. Logins and webhooks each get
// their own limiter, so a misconfigured webhook sender retrying with a wrong
// password cannot lock the sign-in screen.
type failureLimiter struct {
	mu       sync.Mutex
	failures []time.Time
}

const (
	failureWindow = 15 * time.Minute
	failureLimit  = 10
)

// blocked reports whether the caller must be refused before any credential
// check runs — which is also what keeps bcrypt from being a free CPU lever.
func (l *failureLimiter) blocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune()
	return len(l.failures) >= failureLimit
}

func (l *failureLimiter) recordFailure() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune()
	l.failures = append(l.failures, time.Now())
}

func (l *failureLimiter) prune() {
	cutoff := time.Now().Add(-failureWindow)
	kept := l.failures[:0]
	for _, at := range l.failures {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	l.failures = kept
}
