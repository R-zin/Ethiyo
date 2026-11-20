// Package breaker implements a small consecutive-failure circuit breaker.
//
// When an upstream keeps failing, continuing to call it only piles up slow
// requests (each waiting for its own timeout) and adds load to a service that
// is already struggling. After Threshold consecutive failures the breaker
// opens and calls fail fast for Cooldown; then a single trial call is let
// through ("half-open"): success closes the breaker, failure re-opens it.
package breaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State of a circuit breaker.
type State string

const (
	Closed   State = "closed"
	Open     State = "open"
	HalfOpen State = "half_open"
)

// ErrOpen is matched (via errors.Is) by every error returned while the
// breaker refuses calls.
var ErrOpen = errors.New("circuit open: upstream is failing; calls are paused")

// OpenError is returned when a call is refused. RetryAfter is how long until
// the breaker will let a trial call through.
type OpenError struct {
	RetryAfter time.Duration
}

func (e *OpenError) Error() string {
	return fmt.Sprintf("%v (retry in %s)", ErrOpen, e.RetryAfter.Round(time.Second))
}

// Is makes errors.Is(err, ErrOpen) true for an *OpenError.
func (e *OpenError) Is(target error) bool { return target == ErrOpen }

// Status is a point-in-time view of the breaker, safe to serialize.
type Status struct {
	State               State      `json:"state"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccess         *time.Time `json:"last_success,omitempty"`
	LastFailure         *time.Time `json:"last_failure,omitempty"`
	RetryAfterSeconds   float64    `json:"retry_after_seconds,omitempty"`
}

// Breaker is safe for concurrent use.
type Breaker struct {
	mu          sync.Mutex
	threshold   int
	cooldown    time.Duration
	state       State
	failures    int
	openedAt    time.Time
	trialActive bool
	lastSuccess time.Time
	lastFailure time.Time
	now         func() time.Time
}

// New creates a breaker. Non-positive arguments fall back to 5 failures and
// a 30s cooldown.
func New(threshold int, cooldown time.Duration) *Breaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Breaker{threshold: threshold, cooldown: cooldown, state: Closed, now: time.Now}
}

// Do runs fn if the breaker allows it and records the outcome. isFailure
// decides which errors mean "the upstream is unhealthy" (for example, "bus
// not found" is a healthy answer); nil treats every error as a failure.
func (b *Breaker) Do(fn func() error, isFailure func(error) bool) error {
	if err := b.allow(); err != nil {
		return err
	}
	err := fn()
	b.record(err != nil && (isFailure == nil || isFailure(err)))
	return err
}

func (b *Breaker) allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Open:
		elapsed := b.now().Sub(b.openedAt)
		if elapsed < b.cooldown {
			return &OpenError{RetryAfter: b.cooldown - elapsed}
		}
		// Cooldown over: this caller becomes the single trial call.
		b.state = HalfOpen
		b.trialActive = true
		return nil
	case HalfOpen:
		if b.trialActive {
			return &OpenError{RetryAfter: time.Second}
		}
		b.trialActive = true
		return nil
	default:
		return nil
	}
}

func (b *Breaker) record(failed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	if !failed {
		b.lastSuccess = now
		b.failures = 0
		b.state = Closed
		b.trialActive = false
		return
	}

	b.lastFailure = now
	b.failures++
	if b.state == HalfOpen || b.failures >= b.threshold {
		b.state = Open
		b.openedAt = now
		b.trialActive = false
	}
}

// Status reports the effective state: an open breaker whose cooldown has
// elapsed is reported as half-open, because the next call will be a trial.
func (b *Breaker) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()

	st := Status{State: b.state, ConsecutiveFailures: b.failures}
	if b.state == Open {
		remaining := b.cooldown - b.now().Sub(b.openedAt)
		if remaining > 0 {
			st.RetryAfterSeconds = remaining.Round(100 * time.Millisecond).Seconds()
		} else {
			st.State = HalfOpen
		}
	}
	if !b.lastSuccess.IsZero() {
		t := b.lastSuccess
		st.LastSuccess = &t
	}
	if !b.lastFailure.IsZero() {
		t := b.lastFailure
		st.LastFailure = &t
	}
	return st
}
