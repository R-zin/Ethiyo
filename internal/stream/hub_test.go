package stream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

// fakeSource returns a programmable snapshot and counts fetches per bus.
type fakeSource struct {
	mu       sync.Mutex
	recorded time.Time
	err      error
	fetches  map[string]int
	inflight int32
}

func newFakeSource() *fakeSource {
	return &fakeSource{recorded: time.Unix(1000, 0), fetches: map[string]int{}}
}

func (f *fakeSource) fetch(ctx context.Context, code string) (*models.BusTrackingInfo, error) {
	atomic.AddInt32(&f.inflight, 1)
	defer atomic.AddInt32(&f.inflight, -1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches[code]++
	if f.err != nil {
		return nil, f.err
	}
	return &models.BusTrackingInfo{
		BusCode: code,
		Live:    &models.BusLiveInfo{BusCode: code, RecordedAt: f.recorded},
	}, nil
}

func (f *fakeSource) advance() {
	f.mu.Lock()
	f.recorded = f.recorded.Add(30 * time.Second)
	f.mu.Unlock()
}

func (f *fakeSource) setErr(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

func (f *fakeSource) count(code string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fetches[code]
}

func recv(t *testing.T, s *Subscription, within time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-s.C:
		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}
		return ev
	case <-time.After(within):
		t.Fatal("timed out waiting for event")
	}
	return Event{}
}

func expectNoEvent(t *testing.T, s *Subscription, within time.Duration) {
	t.Helper()
	select {
	case ev := <-s.C:
		t.Fatalf("unexpected event: %+v", ev)
	case <-time.After(within):
	}
}

func TestOnePollerFeedsAllSubscribers(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 20*time.Millisecond, 10)
	defer hub.Close()

	subs := make([]*Subscription, 5)
	for i := range subs {
		s, err := hub.Subscribe("KS602")
		if err != nil {
			t.Fatal(err)
		}
		subs[i] = s
	}
	if topics, n := hub.Stats(); topics != 1 || n != 5 {
		t.Fatalf("Stats = %d topics, %d subs; want 1, 5", topics, n)
	}

	for _, s := range subs {
		if ev := recv(t, s, time.Second); ev.Info == nil || ev.Info.BusCode != "KS602" {
			t.Fatalf("bad event %+v", ev)
		}
	}
	// Several poll intervals pass; fetches stay ~1 per interval, not per subscriber.
	time.Sleep(100 * time.Millisecond)
	if n := src.count("KS602"); n > 8 {
		t.Errorf("fetches = %d over ~6 intervals for 5 subscribers; poller is not shared", n)
	}
}

func TestPublishesOnlyOnChange(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 15*time.Millisecond, 10)
	defer hub.Close()

	s, _ := hub.Subscribe("B1")
	recv(t, s, time.Second)
	expectNoEvent(t, s, 80*time.Millisecond) // unchanged snapshots are suppressed

	src.advance()
	if ev := recv(t, s, time.Second); !ev.Info.Live.RecordedAt.Equal(time.Unix(1030, 0)) {
		t.Errorf("got recorded_at %v, want the advanced fix", ev.Info.Live.RecordedAt)
	}
}

func TestErrorsArePublishedAndRecovery(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 15*time.Millisecond, 10)
	defer hub.Close()

	s, _ := hub.Subscribe("B1")
	recv(t, s, time.Second)

	boom := errors.New("upstream down")
	src.setErr(boom)
	if ev := recv(t, s, time.Second); !errors.Is(ev.Err, boom) {
		t.Fatalf("expected error event, got %+v", ev)
	}
	expectNoEvent(t, s, 60*time.Millisecond) // a persisting error is not re-sent

	src.setErr(nil)
	if ev := recv(t, s, time.Second); ev.Err != nil || ev.Info == nil {
		t.Fatalf("expected recovery event, got %+v", ev)
	}
}

func TestLastUnsubscribeStopsPoller(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 10*time.Millisecond, 10)
	defer hub.Close()

	a, _ := hub.Subscribe("B1")
	b, _ := hub.Subscribe("B1")
	a.Close()
	a.Close() // idempotent
	if topics, n := hub.Stats(); topics != 1 || n != 1 {
		t.Fatalf("after one leaves: %d topics, %d subs; want 1, 1", topics, n)
	}
	if _, ok := <-a.C; ok {
		t.Error("closed subscription channel should be closed")
	}

	b.Close()
	if topics, n := hub.Stats(); topics != 0 || n != 0 {
		t.Fatalf("after all leave: %d topics, %d subs; want 0, 0", topics, n)
	}
	time.Sleep(30 * time.Millisecond)
	before := src.count("B1")
	time.Sleep(60 * time.Millisecond)
	if after := src.count("B1"); after != before {
		t.Errorf("poller kept fetching after last unsubscribe (%d → %d)", before, after)
	}
}

func TestCapacityLimit(t *testing.T) {
	hub := NewHub(newFakeSource().fetch, time.Hour, 2)
	defer hub.Close()

	a, err1 := hub.Subscribe("A")
	_, err2 := hub.Subscribe("B")
	_, err3 := hub.Subscribe("C")
	if err1 != nil || err2 != nil || !errors.Is(err3, ErrCapacity) {
		t.Fatalf("errs = %v, %v, %v; want nil, nil, ErrCapacity", err1, err2, err3)
	}
	a.Close()
	if _, err := hub.Subscribe("C"); err != nil {
		t.Errorf("capacity should free up after unsubscribe, got %v", err)
	}
}

func TestSlowSubscriberGetsLatestWithoutBlockingOthers(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 10*time.Millisecond, 10)
	defer hub.Close()

	slow, _ := hub.Subscribe("B1") // never reads until the end
	fast, _ := hub.Subscribe("B1")

	for i := 0; i < 4; i++ {
		recv(t, fast, time.Second)
		src.advance()
	}
	last := recv(t, fast, time.Second).Info.Live.RecordedAt

	// The slow subscriber holds at most one pending event: the newest.
	got := recv(t, slow, time.Second)
	if got.Info.Live.RecordedAt.Before(last) {
		t.Errorf("slow subscriber got %v, want latest (≥ %v)", got.Info.Live.RecordedAt, last)
	}
}

func TestCloseShutsDownEverything(t *testing.T) {
	src := newFakeSource()
	hub := NewHub(src.fetch, 5*time.Millisecond, 10)

	s1, _ := hub.Subscribe("A")
	s2, _ := hub.Subscribe("B")
	hub.Close()
	hub.Close() // idempotent

	for _, s := range []*Subscription{s1, s2} {
		for range s.C { // drain until closed
		}
		s.Close() // closing after hub shutdown must not panic
	}
	if _, err := hub.Subscribe("A"); !errors.Is(err, ErrClosed) {
		t.Errorf("Subscribe after Close = %v, want ErrClosed", err)
	}
	if atomic.LoadInt32(&src.inflight) != 0 {
		t.Error("Close returned while a poller was still fetching")
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	hub := NewHub(newFakeSource().fetch, time.Millisecond, 1000)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				s, err := hub.Subscribe(string(rune('A' + (g+i)%4)))
				if err != nil {
					t.Errorf("Subscribe: %v", err)
					return
				}
				select {
				case <-s.C:
				case <-time.After(time.Millisecond):
				}
				s.Close()
			}
		}(g)
	}
	wg.Wait()
	hub.Close()
	if topics, n := hub.Stats(); topics != 0 || n != 0 {
		t.Errorf("leaked %d topics, %d subs", topics, n)
	}
}
