// Package stream fans live bus positions out to many subscribers while
// polling the upstream only once per bus, regardless of how many clients are
// watching it.
package stream

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

var (
	// ErrCapacity is returned when the hub is at its subscriber limit.
	ErrCapacity = errors.New("live stream capacity reached")
	// ErrClosed is returned when subscribing to a hub that has shut down.
	ErrClosed = errors.New("live stream hub is closed")
)

// Fetcher loads the latest tracking snapshot for a bus.
type Fetcher func(ctx context.Context, busCode string) (*models.BusTrackingInfo, error)

// Event is a single update delivered to subscribers. Exactly one of Info and
// Err is set.
type Event struct {
	Info *models.BusTrackingInfo
	Err  error
}

// Subscription receives events for one bus. C is closed when the subscription
// or the hub is closed.
type Subscription struct {
	C     <-chan Event
	ch    chan Event
	hub   *Hub
	topic *topic
}

// Close unsubscribes. It is safe to call more than once.
func (s *Subscription) Close() { s.hub.unsubscribe(s) }

type topic struct {
	code    string
	subs    map[*Subscription]struct{}
	cancel  context.CancelFunc
	lastKey string
}

// Hub multiplexes per-bus upstream pollers to subscribers.
type Hub struct {
	fetch    Fetcher
	interval time.Duration
	maxSubs  int

	mu     sync.Mutex
	topics map[string]*topic
	nsubs  int
	closed bool
	wg     sync.WaitGroup
}

// NewHub creates a hub that polls each watched bus every interval. maxSubs
// bounds the total number of concurrent subscribers across all buses.
func NewHub(fetch Fetcher, interval time.Duration, maxSubs int) *Hub {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if maxSubs <= 0 {
		maxSubs = 500
	}
	return &Hub{
		fetch:    fetch,
		interval: interval,
		maxSubs:  maxSubs,
		topics:   make(map[string]*topic),
	}
}

// Subscribe registers interest in busCode. The first subscriber for a bus
// starts its poller; the last one to leave stops it.
func (h *Hub) Subscribe(busCode string) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil, ErrClosed
	}
	if h.nsubs >= h.maxSubs {
		return nil, ErrCapacity
	}

	t, ok := h.topics[busCode]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		t = &topic{code: busCode, subs: make(map[*Subscription]struct{}), cancel: cancel}
		h.topics[busCode] = t
		h.wg.Add(1)
		go h.poll(ctx, t)
	}

	ch := make(chan Event, 1)
	sub := &Subscription{C: ch, ch: ch, hub: h, topic: t}
	t.subs[sub] = struct{}{}
	h.nsubs++
	return sub, nil
}

func (h *Hub) unsubscribe(s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := s.topic.subs[s]; !ok {
		return // already closed (directly or by hub shutdown)
	}
	delete(s.topic.subs, s)
	h.nsubs--
	close(s.ch)

	if len(s.topic.subs) == 0 {
		s.topic.cancel()
		if h.topics[s.topic.code] == s.topic {
			delete(h.topics, s.topic.code)
		}
	}
}

// Close stops every poller and closes every subscription. It blocks until all
// pollers have exited and is safe to call more than once.
func (h *Hub) Close() {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		for code, t := range h.topics {
			t.cancel()
			for s := range t.subs {
				delete(t.subs, s)
				close(s.ch)
			}
			delete(h.topics, code)
		}
		h.nsubs = 0
	}
	h.mu.Unlock()
	h.wg.Wait()
}

// Stats reports the number of buses being polled and total subscribers.
func (h *Hub) Stats() (topics, subscribers int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.topics), h.nsubs
}

func (h *Hub) poll(ctx context.Context, t *topic) {
	defer h.wg.Done()
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		info, err := h.fetch(ctx, t.code)
		if ctx.Err() != nil {
			return // unsubscribed or shut down mid-fetch
		}
		h.publish(t, Event{Info: info, Err: err})
	}
}

// eventKey identifies an event's observable state, so unchanged snapshots
// are not re-sent every poll.
func eventKey(ev Event) string {
	if ev.Err != nil {
		return "err:" + ev.Err.Error()
	}
	if ev.Info == nil || ev.Info.Live == nil {
		return "empty"
	}
	stale := "fresh"
	if ev.Info.Live.Stale {
		stale = "stale"
	}
	return ev.Info.Live.RecordedAt.UTC().Format(time.RFC3339Nano) + "|" + stale
}

func (h *Hub) publish(t *topic, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := eventKey(ev)
	if key == t.lastKey {
		return
	}
	t.lastKey = key

	for s := range t.subs {
		// Latest-wins delivery: a slow subscriber only ever has the newest
		// event pending, and never blocks the poller or other subscribers.
		select {
		case s.ch <- ev:
		default:
			select {
			case <-s.ch:
			default:
			}
			select {
			case s.ch <- ev:
			default:
			}
		}
	}
}
