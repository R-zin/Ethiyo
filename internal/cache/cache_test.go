package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock lets tests advance time deterministically.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

func newWithClock[K comparable, V any](ttl time.Duration, max int) (*TTL[K, V], *fakeClock) {
	c := New[K, V](ttl, max)
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c.now = clk.Now
	return c, clk
}

func TestGetSetAndExpiry(t *testing.T) {
	c, clk := newWithClock[string, int](time.Second, 10)
	c.Set("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v,%v want 1,true", v, ok)
	}
	clk.Advance(999 * time.Millisecond)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("entry expired too early")
	}
	clk.Advance(time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("entry should expire exactly at TTL")
	}
	if c.Len() != 0 {
		t.Errorf("expired entry should be removed on read, Len = %d", c.Len())
	}
}

func TestEvictionBoundsSize(t *testing.T) {
	c, clk := newWithClock[int, int](time.Minute, 3)
	for i := 0; i < 3; i++ {
		c.Set(i, i)
		clk.Advance(time.Second) // key 0 expires first
	}
	c.Set(99, 99)
	if c.Len() != 3 {
		t.Fatalf("Len = %d, want 3", c.Len())
	}
	if _, ok := c.Get(0); ok {
		t.Error("entry closest to expiry should have been evicted")
	}
	if _, ok := c.Get(99); !ok {
		t.Error("new entry missing")
	}
	// Overwriting an existing key never evicts.
	c.Set(99, 100)
	if c.Len() != 3 {
		t.Errorf("overwrite changed Len to %d", c.Len())
	}
}

func TestEvictionPrefersExpired(t *testing.T) {
	c, clk := newWithClock[int, int](time.Second, 2)
	c.Set(1, 1)
	clk.Advance(2 * time.Second) // 1 is now expired
	c.Set(2, 2)
	c.Set(3, 3)
	if _, ok := c.Get(2); !ok {
		t.Error("live entry 2 evicted while an expired entry existed")
	}
}

func TestGetOrLoadCachesAndReportsHit(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	var loads int32
	load := func(context.Context) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "v", nil
	}
	v, hit, err := c.GetOrLoad(context.Background(), "k", load)
	if err != nil || v != "v" || hit {
		t.Fatalf("first load = %q,%v,%v", v, hit, err)
	}
	v, hit, err = c.GetOrLoad(context.Background(), "k", load)
	if err != nil || v != "v" || !hit {
		t.Fatalf("second load = %q,%v,%v; want cache hit", v, hit, err)
	}
	if loads != 1 {
		t.Errorf("loads = %d, want 1", loads)
	}
}

func TestGetOrLoadDoesNotCacheErrors(t *testing.T) {
	c := New[string, int](time.Minute, 10)
	boom := errors.New("boom")
	var loads int32
	load := func(context.Context) (int, error) {
		if atomic.AddInt32(&loads, 1) == 1 {
			return 0, boom
		}
		return 7, nil
	}
	if _, _, err := c.GetOrLoad(context.Background(), "k", load); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if v, _, err := c.GetOrLoad(context.Background(), "k", load); err != nil || v != 7 {
		t.Fatalf("retry = %v,%v want 7,nil", v, err)
	}
}

func TestGetOrLoadCoalescesConcurrentCallers(t *testing.T) {
	c := New[string, int](time.Minute, 10)
	var loads int32
	release := make(chan struct{})
	load := func(context.Context) (int, error) {
		atomic.AddInt32(&loads, 1)
		<-release
		return 42, nil
	}

	const callers = 50
	var wg sync.WaitGroup
	results := make(chan int, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _, err := c.GetOrLoad(context.Background(), "k", load)
			if err != nil {
				t.Errorf("GetOrLoad: %v", err)
			}
			results <- v
		}()
	}
	time.Sleep(50 * time.Millisecond) // let callers pile up on the in-flight load
	close(release)
	wg.Wait()
	close(results)

	if loads != 1 {
		t.Errorf("loads = %d, want exactly 1 for %d concurrent callers", loads, callers)
	}
	for v := range results {
		if v != 42 {
			t.Errorf("caller got %d, want 42", v)
		}
	}
}

func TestGetOrLoadCallerCancellationDoesNotAbortSharedLoad(t *testing.T) {
	c := New[string, int](time.Minute, 10)
	release := make(chan struct{})
	loadCtxErr := make(chan error, 1)
	load := func(ctx context.Context) (int, error) {
		<-release
		loadCtxErr <- ctx.Err()
		return 1, nil
	}

	impatient, cancel := context.WithCancel(context.Background())
	firstErr := make(chan error, 1)
	go func() {
		_, _, err := c.GetOrLoad(impatient, "k", load)
		firstErr <- err
	}()
	time.Sleep(20 * time.Millisecond)

	patientDone := make(chan int, 1)
	go func() {
		v, _, _ := c.GetOrLoad(context.Background(), "k", load)
		patientDone <- v
	}()
	time.Sleep(20 * time.Millisecond)

	cancel()
	if err := <-firstErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("impatient caller err = %v, want context.Canceled", err)
	}
	close(release)

	if v := <-patientDone; v != 1 {
		t.Errorf("patient caller got %d, want 1", v)
	}
	if err := <-loadCtxErr; err != nil {
		t.Errorf("shared load saw ctx error %v; it must be detached from the leader's cancellation", err)
	}
}

func TestGetOrLoadPreservesContextValues(t *testing.T) {
	type key struct{}
	c := New[string, string](time.Minute, 10)
	ctx := context.WithValue(context.Background(), key{}, "req-123")
	v, _, err := c.GetOrLoad(ctx, "k", func(ctx context.Context) (string, error) {
		s, _ := ctx.Value(key{}).(string)
		return s, nil
	})
	if err != nil || v != "req-123" {
		t.Errorf("load saw value %q (err %v), want req-123", v, err)
	}
}

func TestZeroTTLDisablesStorageButCoalesces(t *testing.T) {
	c := New[string, int](0, 10)
	var loads int32
	load := func(context.Context) (int, error) {
		atomic.AddInt32(&loads, 1)
		return 1, nil
	}
	for i := 0; i < 3; i++ {
		if _, hit, _ := c.GetOrLoad(context.Background(), "k", load); hit {
			t.Fatal("zero TTL must never report a cache hit")
		}
	}
	if loads != 3 {
		t.Errorf("loads = %d, want 3 (no storage)", loads)
	}
	c.Set("k", 5)
	if c.Len() != 0 {
		t.Error("Set must be a no-op with zero TTL")
	}
}

func TestPanickingLoadReleasesWaiters(t *testing.T) {
	c := New[string, int](time.Minute, 10)
	done := make(chan error, 1)
	go func() {
		_, _, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) {
			panic("kaboom")
		})
		done <- err
	}()
	select {
	case err := <-done:
		var pe *PanicError
		if !errors.As(err, &pe) {
			t.Fatalf("err = %v, want *PanicError", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter blocked forever after a panicking load")
	}
	// The key must be loadable again afterwards.
	if v, _, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 3, nil }); err != nil || v != 3 {
		t.Errorf("reload after panic = %v,%v", v, err)
	}
}
