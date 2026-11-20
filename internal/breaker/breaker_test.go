package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestBreaker(threshold int, cooldown time.Duration) (*Breaker, *clock) {
	b := New(threshold, cooldown)
	clk := &clock{t: time.Unix(1_700_000_000, 0)}
	b.now = clk.Now
	return b, clk
}

func fail() error    { return errBoom }
func succeed() error { return nil }

func TestOpensAfterConsecutiveFailures(t *testing.T) {
	b, _ := newTestBreaker(3, time.Minute)
	for i := 0; i < 3; i++ {
		if err := b.Do(fail, nil); !errors.Is(err, errBoom) {
			t.Fatalf("call %d: err = %v, want the upstream error", i, err)
		}
	}
	if b.Status().State != Open {
		t.Fatalf("state = %s, want open after 3 failures", b.Status().State)
	}

	calls := 0
	err := b.Do(func() error { calls++; return nil }, nil)
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("err = %v, want ErrOpen", err)
	}
	if calls != 0 {
		t.Error("an open breaker must not call the upstream")
	}
	var oe *OpenError
	if !errors.As(err, &oe) || oe.RetryAfter != time.Minute {
		t.Errorf("RetryAfter = %v, want 1m", oe)
	}
}

func TestSuccessResetsTheFailureCount(t *testing.T) {
	b, _ := newTestBreaker(3, time.Minute)
	_ = b.Do(fail, nil)
	_ = b.Do(fail, nil)
	_ = b.Do(succeed, nil)
	_ = b.Do(fail, nil)
	_ = b.Do(fail, nil)
	if st := b.Status(); st.State != Closed || st.ConsecutiveFailures != 2 {
		t.Errorf("status = %+v; failures must be consecutive to open the breaker", st)
	}
}

func TestIsFailureClassifier(t *testing.T) {
	notFound := errors.New("not found")
	b, _ := newTestBreaker(2, time.Minute)
	healthy := func(err error) bool { return !errors.Is(err, notFound) }
	for i := 0; i < 5; i++ {
		if err := b.Do(func() error { return notFound }, healthy); !errors.Is(err, notFound) {
			t.Fatalf("err = %v", err)
		}
	}
	if st := b.Status(); st.State != Closed || st.LastSuccess == nil {
		t.Errorf("status = %+v; healthy errors must count as successes", st)
	}
}

func TestHalfOpenTrialClosesOnSuccess(t *testing.T) {
	b, clk := newTestBreaker(1, 10*time.Second)
	_ = b.Do(fail, nil)

	clk.Advance(4 * time.Second)
	var oe *OpenError
	if err := b.Do(succeed, nil); !errors.As(err, &oe) || oe.RetryAfter != 6*time.Second {
		t.Fatalf("during cooldown: err = %v, want OpenError with 6s left", err)
	}
	if got := b.Status().RetryAfterSeconds; got != 6 {
		t.Errorf("Status.RetryAfterSeconds = %v, want 6", got)
	}

	clk.Advance(6 * time.Second)
	if st := b.Status(); st.State != HalfOpen {
		t.Errorf("after cooldown, status = %s, want half_open", st.State)
	}
	if err := b.Do(succeed, nil); err != nil {
		t.Fatalf("trial call refused: %v", err)
	}
	if st := b.Status(); st.State != Closed || st.ConsecutiveFailures != 0 {
		t.Errorf("after successful trial: %+v, want closed", st)
	}
}

func TestHalfOpenTrialReopensOnFailure(t *testing.T) {
	b, clk := newTestBreaker(3, 10*time.Second)
	for i := 0; i < 3; i++ {
		_ = b.Do(fail, nil)
	}
	clk.Advance(10 * time.Second)

	// A single failed trial re-opens immediately (no need for 3 more).
	_ = b.Do(fail, nil)
	if st := b.Status(); st.State != Open || st.RetryAfterSeconds != 10 {
		t.Errorf("after failed trial: %+v, want open with a fresh 10s cooldown", st)
	}
}

func TestOnlyOneTrialCallAtATime(t *testing.T) {
	b, clk := newTestBreaker(1, time.Second)
	_ = b.Do(fail, nil)
	clk.Advance(time.Second)

	release := make(chan struct{})
	var started int32
	trialDone := make(chan error, 1)
	go func() {
		trialDone <- b.Do(func() error {
			atomic.AddInt32(&started, 1)
			<-release
			return nil
		}, nil)
	}()
	for atomic.LoadInt32(&started) == 0 {
		time.Sleep(time.Millisecond)
	}

	// While the trial is in flight, other callers still fail fast.
	if err := b.Do(succeed, nil); !errors.Is(err, ErrOpen) {
		t.Fatalf("concurrent call during trial: err = %v, want ErrOpen", err)
	}
	close(release)
	if err := <-trialDone; err != nil {
		t.Fatalf("trial: %v", err)
	}
	if err := b.Do(succeed, nil); err != nil {
		t.Errorf("after the trial succeeded: %v", err)
	}
}

func TestStatusTimestamps(t *testing.T) {
	b, clk := newTestBreaker(5, time.Second)
	if st := b.Status(); st.LastSuccess != nil || st.LastFailure != nil {
		t.Fatalf("fresh breaker should have no timestamps: %+v", st)
	}
	_ = b.Do(succeed, nil)
	clk.Advance(time.Second)
	_ = b.Do(fail, nil)
	st := b.Status()
	if st.LastSuccess == nil || st.LastFailure == nil || !st.LastFailure.After(*st.LastSuccess) {
		t.Errorf("timestamps = %+v", st)
	}
}

func TestDefaultsAndConcurrency(t *testing.T) {
	b := New(0, 0)
	if b.threshold != 5 || b.cooldown != 30*time.Second {
		t.Fatalf("defaults = %d/%v", b.threshold, b.cooldown)
	}
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if (g+i)%3 == 0 {
					_ = b.Do(fail, nil)
				} else {
					_ = b.Do(succeed, nil)
				}
				b.Status()
			}
		}(g)
	}
	wg.Wait()
}
