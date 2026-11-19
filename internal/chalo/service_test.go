package chalo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// livePayloadAt renders a Chalo vehicle-tracking payload for a position.
func livePayloadAt(lat, lon float64, ts time.Time) string {
	return fmt.Sprintf(`{
	  "gpsData": {"vehicleCode": "KL15A3159", "operator": {"name": "KSRTC"}, "city": {"liveTracking": true}},
	  "sessionData": {"data": {
	    "currentInfo": {"lt": %f, "ln": %f, "pSp": 30, "tS": %d},
	    "nextStop": {"name": "Next"}, "previousStop": {"name": "Prev"},
	    "routeDetails": {"routeName": "Route <b>X</b>"}
	  }}
	}`, lat, lon, ts.UnixMilli())
}

const unknownVehicleBody = `{"error":"gps data is not present for this vehicle"}`

// scriptedUpstream serves a programmable sequence of live-tracking responses
// and counts upstream calls.
type scriptedUpstream struct {
	mu     sync.Mutex
	status int
	body   string
	delay  time.Duration
	calls  int32
}

func (u *scriptedUpstream) set(status int, body string) {
	u.mu.Lock()
	u.status, u.body = status, body
	u.mu.Unlock()
}

func (u *scriptedUpstream) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&u.calls, 1)
	u.mu.Lock()
	status, body, delay := u.status, u.body, u.delay
	u.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
}

func (u *scriptedUpstream) Calls() int { return int(atomic.LoadInt32(&u.calls)) }

type recordingObserver struct {
	mu       sync.Mutex
	cache    map[string][2]int // [hits, misses]
	upstream map[string][]error
}

func newRecordingObserver() *recordingObserver {
	return &recordingObserver{cache: map[string][2]int{}, upstream: map[string][]error{}}
}

func (o *recordingObserver) ObserveCache(name string, hit bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	c := o.cache[name]
	if hit {
		c[0]++
	} else {
		c[1]++
	}
	o.cache[name] = c
}

func (o *recordingObserver) ObserveUpstream(op string, _ time.Duration, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.upstream[op] = append(o.upstream[op], err)
}

func newTestService(t *testing.T, opts Options) (*ChaloService, *scriptedUpstream) {
	t.Helper()
	up := &scriptedUpstream{status: http.StatusAccepted, body: livePayloadAt(10, 76, time.Now())}
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(up)
	return NewServiceWithOptions(client, nil, opts), up
}

func TestLiveCacheServesRepeatedRequestsFromMemory(t *testing.T) {
	obs := newRecordingObserver()
	svc, up := newTestService(t, Options{LiveCacheTTL: time.Minute, Observer: obs})

	for i := 0; i < 5; i++ {
		if _, err := svc.GetBusTracking(context.Background(), "KS602"); err != nil {
			t.Fatalf("GetBusTracking: %v", err)
		}
	}
	if up.Calls() != 1 {
		t.Errorf("upstream calls = %d, want 1 within the cache TTL", up.Calls())
	}
	if got := obs.cache["live"]; got != [2]int{4, 1} {
		t.Errorf("cache hits/misses = %v, want [4 1]", got)
	}
	if errs := obs.upstream[OpLiveTracking]; len(errs) != 1 || errs[0] != nil {
		t.Errorf("upstream observations = %v, want one success", errs)
	}
}

func TestLiveRequestsCoalesceUnderConcurrency(t *testing.T) {
	svc, up := newTestService(t, Options{}) // caching off: coalescing only
	up.delay = 100 * time.Millisecond

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GetBusTracking(context.Background(), "KS602"); err != nil {
				t.Errorf("GetBusTracking: %v", err)
			}
		}()
	}
	wg.Wait()
	if up.Calls() != 1 {
		t.Errorf("upstream calls = %d, want 1 for a burst of concurrent identical requests", up.Calls())
	}
}

func TestCachedValueIsIsolatedFromCallers(t *testing.T) {
	svc, _ := newTestService(t, Options{LiveCacheTTL: time.Minute})
	first, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatal(err)
	}
	first.Live.Latitude = -99
	first.Live.RouteName = "mutated"

	second, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatal(err)
	}
	if second.Live.Latitude == -99 || second.Live.RouteName == "mutated" {
		t.Error("mutating a response leaked into the shared cache entry")
	}
}

func TestFreshnessAgeAndStaleFlag(t *testing.T) {
	svc, up := newTestService(t, Options{StaleAfter: time.Minute})
	recorded := time.Now().Add(-90 * time.Second)
	up.set(http.StatusAccepted, livePayloadAt(10, 76, recorded))
	svc.now = func() time.Time { return recorded.Add(90 * time.Second) }

	info, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(info.Live.AgeSeconds-90) > 0.11 {
		t.Errorf("AgeSeconds = %v, want 90", info.Live.AgeSeconds)
	}
	if !info.Live.Stale {
		t.Error("90s-old fix should be stale with StaleAfter=1m")
	}

	svc.now = func() time.Time { return recorded.Add(10 * time.Second) }
	info, _ = svc.GetBusTracking(context.Background(), "KS602")
	if info.Live.Stale || math.Abs(info.Live.AgeSeconds-10) > 0.11 {
		t.Errorf("10s-old fix: age=%v stale=%v; want 10,false", info.Live.AgeSeconds, info.Live.Stale)
	}

	// A vehicle clock running ahead must not produce a negative age.
	svc.now = func() time.Time { return recorded.Add(-5 * time.Second) }
	info, _ = svc.GetBusTracking(context.Background(), "KS602")
	if info.Live.AgeSeconds != 0 {
		t.Errorf("future fix age = %v, want 0", info.Live.AgeSeconds)
	}
}

func TestHeadingDerivedFromSuccessiveFixes(t *testing.T) {
	svc, up := newTestService(t, Options{}) // no cache so each call hits upstream
	t0 := time.Now().Add(-time.Minute)

	up.set(http.StatusAccepted, livePayloadAt(10, 76, t0))
	info, _ := svc.GetBusTracking(context.Background(), "KS602")
	if info.Live.Heading != nil {
		t.Fatalf("first fix must not report a heading, got %v", *info.Live.Heading)
	}

	up.set(http.StatusAccepted, livePayloadAt(10, 76.01, t0.Add(30*time.Second))) // due east
	info, _ = svc.GetBusTracking(context.Background(), "KS602")
	if info.Live.Heading == nil || math.Abs(*info.Live.Heading-90) > 0.1 {
		t.Fatalf("heading = %v, want ~90 (east)", info.Live.Heading)
	}
}

func TestGetBusHistoryTrail(t *testing.T) {
	svc, up := newTestService(t, Options{})
	t0 := time.Now().Add(-2 * time.Minute)

	// Unknown bus with no trail: 404 semantics.
	up.set(http.StatusAccepted, unknownVehicleBody)
	if _, err := svc.GetBusHistory(context.Background(), "ZZZ1"); !errors.Is(err, ErrBusNotFound) {
		t.Fatalf("unknown bus: err = %v, want ErrBusNotFound", err)
	}

	for i := 0; i < 3; i++ {
		up.set(http.StatusAccepted, livePayloadAt(10+float64(i)*0.01, 76, t0.Add(time.Duration(i)*30*time.Second)))
		if _, err := svc.GetBusTracking(context.Background(), "KS602"); err != nil {
			t.Fatal(err)
		}
	}
	h, err := svc.GetBusHistory(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("GetBusHistory: %v", err)
	}
	if len(h.Points) != 3 {
		t.Fatalf("points = %d, want 3 (the history call must not duplicate the last fix)", len(h.Points))
	}
	if h.DistanceMeters < 2000 || h.DistanceMeters > 2500 {
		t.Errorf("distance = %v m, want ~2224", h.DistanceMeters)
	}

	// The bus goes dark: its last known trail is still served.
	up.set(http.StatusAccepted, unknownVehicleBody)
	h, err = svc.GetBusHistory(context.Background(), "KS602")
	if err != nil || len(h.Points) != 3 {
		t.Errorf("dark bus: points=%v err=%v; want last trail, nil", h, err)
	}

	// But an upstream outage is reported, not masked by the old trail.
	up.set(http.StatusBadGateway, "down")
	if _, err := svc.GetBusHistory(context.Background(), "KS602"); !errors.Is(err, ErrUpstreamService) {
		t.Errorf("outage: err = %v, want ErrUpstreamService", err)
	}
}

func TestUnknownBusIsNotCountedAsUpstreamFailure(t *testing.T) {
	obs := newRecordingObserver()
	svc, up := newTestService(t, Options{Observer: obs})

	up.set(http.StatusAccepted, unknownVehicleBody)
	_, _ = svc.GetBusTracking(context.Background(), "ZZZ1")
	up.set(http.StatusBadGateway, "down")
	_, _ = svc.GetBusTracking(context.Background(), "ZZZ1")

	errs := obs.upstream[OpLiveTracking]
	if len(errs) != 2 || errs[0] != nil || !errors.Is(errs[1], ErrUpstreamService) {
		t.Errorf("upstream observations = %v; want [nil, ErrUpstreamService]", errs)
	}
}

func TestGetBusRouteIsCached(t *testing.T) {
	scraper, wantRoute, cleanup := newRouteOnlyFixture(t)
	defer cleanup()

	obs := newRecordingObserver()
	client := NewClient(scraper.baseURL, 5*time.Second)
	svc := NewServiceWithOptions(client, scraper, Options{RouteCacheTTL: time.Minute, Observer: obs})

	first, err := svc.GetBusRoute(context.Background(), "X")
	if err != nil {
		t.Fatalf("GetBusRoute: %v", err)
	}
	start := time.Now()
	second, err := svc.GetBusRoute(context.Background(), "X")
	if err != nil {
		t.Fatalf("GetBusRoute (cached): %v", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("cached route lookup took %v; should not launch a browser", time.Since(start))
	}
	if first.RouteURL != wantRoute || second.RouteURL != wantRoute {
		t.Errorf("route URLs = %q, %q; want %q", first.RouteURL, second.RouteURL, wantRoute)
	}
	if got := obs.cache["route"]; got != [2]int{1, 1} {
		t.Errorf("route cache hits/misses = %v, want [1 1]", got)
	}
	if n := len(obs.upstream[OpRouteDiscovery]); n != 1 {
		t.Errorf("route discoveries = %d, want 1", n)
	}
}
