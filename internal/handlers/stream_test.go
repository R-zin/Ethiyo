package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/stream"
	"github.com/gin-gonic/gin"
)

// movingBus is a service stub whose position advances on demand.
type movingBus struct {
	mu       sync.Mutex
	recorded time.Time
	err      error
}

func (m *movingBus) get(ctx context.Context, code string) (*models.BusTrackingInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if code == "GONE1" {
		return nil, chalo.ErrBusNotFound
	}
	if m.err != nil {
		return nil, m.err
	}
	return &models.BusTrackingInfo{
		BusCode:     code,
		TrackingURL: "https://chalo.com/x",
		Live:        &models.BusLiveInfo{BusCode: code, Latitude: 10, Longitude: 76, RecordedAt: m.recorded},
	}, nil
}

func (m *movingBus) advance() {
	m.mu.Lock()
	m.recorded = m.recorded.Add(30 * time.Second)
	m.mu.Unlock()
}

func (m *movingBus) setErr(err error) {
	m.mu.Lock()
	m.err = err
	m.mu.Unlock()
}

type streamFixture struct {
	srv *httptest.Server
	hub *stream.Hub
	bus *movingBus
}

func newStreamFixture(t *testing.T, maxSubs int, serverWriteTimeout time.Duration, opts StreamOptions) *streamFixture {
	t.Helper()
	bus := &movingBus{recorded: time.Unix(1_789_836_570, 0).UTC()}
	svc := &mockChaloService{getBusTrackingFunc: bus.get}
	hub := stream.NewHub(bus.get, 20*time.Millisecond, maxSubs)

	r := gin.New()
	r.GET("/api/v1/bus/:buscode/stream", NewStreamHandler(svc, hub, opts).Stream)

	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = serverWriteTimeout
	srv.Start()
	t.Cleanup(func() {
		hub.Close()
		srv.Close()
	})
	return &streamFixture{srv: srv, hub: hub, bus: bus}
}

type sseEvent struct {
	name, data string
	comment    bool
}

// readEvent reads one SSE frame (event, retry directive, or comment).
func readEvent(t *testing.T, r *bufio.Reader) (sseEvent, error) {
	t.Helper()
	var ev sseEvent
	sawField := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return ev, err
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case line == "":
			if sawField {
				return ev, nil
			}
		case strings.HasPrefix(line, ":"):
			ev.comment, sawField = true, true
		case strings.HasPrefix(line, "event: "):
			ev.name, sawField = strings.TrimPrefix(line, "event: "), true
		case strings.HasPrefix(line, "data: "):
			ev.data, sawField = strings.TrimPrefix(line, "data: "), true
		case strings.HasPrefix(line, "retry: "):
			ev.name, sawField = "retry", true
		}
	}
}

// nextNamed skips heartbeats and returns the next named event.
func nextNamed(t *testing.T, r *bufio.Reader) sseEvent {
	t.Helper()
	for {
		ev, err := readEvent(t, r)
		if err != nil {
			t.Fatalf("reading stream: %v", err)
		}
		if !ev.comment {
			return ev
		}
	}
}

func openStream(t *testing.T, f *streamFixture, code string) (*http.Response, *bufio.Reader, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.srv.URL+"/api/v1/bus/"+code+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("GET stream: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close(); cancel() })
	return resp, bufio.NewReader(resp.Body), cancel
}

func TestStreamDeliversInitialSnapshotAndUpdates(t *testing.T) {
	f := newStreamFixture(t, 10, 0, StreamOptions{Heartbeat: time.Hour})
	resp, r, _ := openStream(t, f, "KS602")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if ev := nextNamed(t, r); ev.name != "retry" {
		t.Fatalf("first frame = %+v, want retry directive", ev)
	}

	ev := nextNamed(t, r)
	var info models.BusTrackingInfo
	if ev.name != eventPosition || json.Unmarshal([]byte(ev.data), &info) != nil || info.BusCode != "KS602" {
		t.Fatalf("initial event = %+v", ev)
	}
	first := info.Live.RecordedAt

	f.bus.advance()
	ev = nextNamed(t, r)
	if ev.name != eventPosition || json.Unmarshal([]byte(ev.data), &info) != nil {
		t.Fatalf("update event = %+v", ev)
	}
	if !info.Live.RecordedAt.After(first) {
		t.Errorf("update recorded_at %v not after %v", info.Live.RecordedAt, first)
	}
}

// TestStreamOutlivesServerWriteTimeout is the key regression guard: the
// server's WriteTimeout (sized for ordinary requests) must not cut streams.
func TestStreamOutlivesServerWriteTimeout(t *testing.T) {
	f := newStreamFixture(t, 10, 150*time.Millisecond, StreamOptions{Heartbeat: 40 * time.Millisecond})
	_, r, _ := openStream(t, f, "KS602")

	deadline := time.Now().Add(600 * time.Millisecond) // 4x the server WriteTimeout
	pings := 0
	for time.Now().Before(deadline) {
		ev, err := readEvent(t, r)
		if err != nil {
			t.Fatalf("stream cut after %d heartbeats: %v", pings, err)
		}
		if ev.comment {
			pings++
		}
	}
	if pings < 5 {
		t.Errorf("only %d heartbeats in 600ms at a 40ms interval", pings)
	}
}

func TestStreamRejectsBeforeStreamingWithJSON(t *testing.T) {
	f := newStreamFixture(t, 1, 0, StreamOptions{})

	cases := map[string]struct {
		status int
		code   string
	}{
		"bad$$code": {http.StatusBadRequest, "INVALID_BUS_CODE"},
		"GONE1":     {http.StatusNotFound, "BUS_NOT_FOUND"},
	}
	for code, want := range cases {
		resp, err := http.Get(f.srv.URL + "/api/v1/bus/" + code + "/stream")
		if err != nil {
			t.Fatal(err)
		}
		var body models.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != want.status || body.Error.Code != want.code {
			t.Errorf("%s: %d %s, want %d %s", code, resp.StatusCode, body.Error.Code, want.status, want.code)
		}
	}
	// The rejected GONE1 request must have released its subscription slot.
	if _, n := f.hub.Stats(); n != 0 {
		t.Errorf("rejected requests leaked %d subscriptions", n)
	}

	// Saturate the single slot, then the next client gets 503 + Retry-After.
	_, r, _ := openStream(t, f, "KS602")
	nextNamed(t, r)
	resp, err := http.Get(f.srv.URL + "/api/v1/bus/KS602/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body models.ErrorResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusServiceUnavailable || body.Error.Code != "STREAM_CAPACITY" {
		t.Errorf("over capacity: %d %s", resp.StatusCode, body.Error.Code)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("capacity rejection should carry Retry-After")
	}
}

func TestStreamEmitsUpstreamErrorEvents(t *testing.T) {
	f := newStreamFixture(t, 10, 0, StreamOptions{Heartbeat: time.Hour})
	_, r, _ := openStream(t, f, "KS602")
	nextNamed(t, r) // retry
	nextNamed(t, r) // initial position

	f.bus.setErr(chalo.ErrUpstreamService)
	ev := nextNamed(t, r)
	var detail models.ErrorDetail
	if ev.name != eventUpstreamError || json.Unmarshal([]byte(ev.data), &detail) != nil || detail.Code != "UPSTREAM_CHALO_ERROR" {
		t.Fatalf("event = %+v, want upstream_error UPSTREAM_CHALO_ERROR", ev)
	}
	// Internal error text must not leak to clients.
	if strings.Contains(ev.data, "status") {
		t.Errorf("error event leaks internals: %s", ev.data)
	}
}

func TestStreamReleasesSubscriptionOnClientDisconnect(t *testing.T) {
	f := newStreamFixture(t, 10, 0, StreamOptions{Heartbeat: 10 * time.Millisecond})
	_, r, cancel := openStream(t, f, "KS602")
	nextNamed(t, r)
	if _, n := f.hub.Stats(); n != 1 {
		t.Fatalf("subscribers = %d, want 1", n)
	}

	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if topics, n := f.hub.Stats(); topics == 0 && n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	topics, n := f.hub.Stats()
	t.Fatalf("after disconnect: %d topics, %d subscribers; want 0, 0", topics, n)
}

func TestStreamEndsWhenHubCloses(t *testing.T) {
	f := newStreamFixture(t, 10, 0, StreamOptions{Heartbeat: time.Hour})
	_, r, _ := openStream(t, f, "KS602")
	nextNamed(t, r)
	nextNamed(t, r)

	f.hub.Close()
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, r)
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not end after hub shutdown")
	}
}

func TestStreamMaxLifetime(t *testing.T) {
	f := newStreamFixture(t, 10, 0, StreamOptions{Heartbeat: time.Hour, MaxLifetime: 100 * time.Millisecond})
	_, r, _ := openStream(t, f, "KS602")
	start := time.Now()
	if _, err := io.Copy(io.Discard, r); err != nil {
		t.Fatalf("stream ended with error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("stream outlived MaxLifetime: %v", elapsed)
	}
}
