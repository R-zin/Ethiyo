package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/stream"
	"github.com/gin-gonic/gin"
)

type fleetEnvelope struct {
	Success bool                 `json:"success"`
	Data    models.FleetResponse `json:"data"`
	Error   models.ErrorDetail   `json:"error"`
}

func newFleetRouter(svc chalo.Service, hub *stream.Hub) *gin.Engine {
	r := gin.New()
	h := NewFleetHandler(svc, hub, StreamOptions{Heartbeat: time.Hour})
	r.GET("/api/v1/buses/track", h.Track)
	r.GET("/api/v1/buses/stream", h.Stream)
	return r
}

func TestFleetTrackPartialSuccessInRequestOrder(t *testing.T) {
	svc := &mockChaloService{
		getBusTrackingFunc: func(ctx context.Context, code string) (*models.BusTrackingInfo, error) {
			switch code {
			case "GONE1":
				return nil, chalo.ErrBusNotFound
			case "DOWN1":
				return nil, fmt.Errorf("%w: status 502", chalo.ErrUpstreamService)
			}
			return &models.BusTrackingInfo{BusCode: code, Live: &models.BusLiveInfo{BusCode: code}}, nil
		},
	}
	hub := stream.NewHub(func(context.Context, string) (*models.BusTrackingInfo, error) { return nil, nil }, time.Hour, 10)
	defer hub.Close()

	w := httptest.NewRecorder()
	newFleetRouter(svc, hub).ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/buses/track?codes=KS602,GONE1,DL1,KS602,DOWN1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var env fleetEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}

	wantOrder := []string{"KS602", "GONE1", "DL1", "DOWN1"}
	if len(env.Data.Buses) != len(wantOrder) {
		t.Fatalf("got %d items, want %d (duplicates collapsed)", len(env.Data.Buses), len(wantOrder))
	}
	for i, it := range env.Data.Buses {
		if it.BusCode != wantOrder[i] {
			t.Errorf("item %d = %s, want %s", i, it.BusCode, wantOrder[i])
		}
		if (it.Tracking == nil) == (it.Error == nil) {
			t.Errorf("%s: exactly one of tracking/error must be set: %+v", it.BusCode, it)
		}
	}
	if e := env.Data.Buses[1].Error; e == nil || e.Code != "BUS_NOT_FOUND" {
		t.Errorf("GONE1 error = %+v", e)
	}
	if e := env.Data.Buses[3].Error; e == nil || e.Code != "UPSTREAM_CHALO_ERROR" || strings.Contains(e.Message, "502") {
		t.Errorf("DOWN1 error = %+v (must be classified, without internal details)", e)
	}
}

func TestFleetTrackValidation(t *testing.T) {
	r := newFleetRouter(&mockChaloService{}, stream.NewHub(nil, time.Hour, 1))
	many := make([]string, models.MaxFleetSize+1)
	for i := range many {
		many[i] = fmt.Sprintf("B%d", i)
	}
	cases := map[string]string{
		"/api/v1/buses/track":                                  "INVALID_BUS_CODE",
		"/api/v1/buses/track?codes=":                           "INVALID_BUS_CODE",
		"/api/v1/buses/track?codes=KS602,bad$code":             "INVALID_BUS_CODE",
		"/api/v1/buses/track?codes=" + strings.Join(many, ","): "TOO_MANY_BUS_CODES",
	}
	for url, wantCode := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		var env fleetEnvelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if w.Code != http.StatusBadRequest || env.Error.Code != wantCode {
			t.Errorf("%s: %d %s, want 400 %s", url, w.Code, env.Error.Code, wantCode)
		}
	}
}

func TestFleetTrackBoundsConcurrency(t *testing.T) {
	var inFlight, peak int32
	svc := &mockChaloService{
		getBusTrackingFunc: func(ctx context.Context, code string) (*models.BusTrackingInfo, error) {
			n := atomic.AddInt32(&inFlight, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return &models.BusTrackingInfo{BusCode: code}, nil
		},
	}
	codes := make([]string, models.MaxFleetSize)
	for i := range codes {
		codes[i] = fmt.Sprintf("B%d", i)
	}
	w := httptest.NewRecorder()
	newFleetRouter(svc, stream.NewHub(nil, time.Hour, 1)).ServeHTTP(w,
		httptest.NewRequest(http.MethodGet, "/api/v1/buses/track?codes="+strings.Join(codes, ","), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if peak > fleetFetchConcurrency {
		t.Errorf("peak concurrent upstream lookups = %d, want ≤ %d", peak, fleetFetchConcurrency)
	}
	if peak < 2 {
		t.Errorf("peak = %d; lookups should run in parallel", peak)
	}
}

func newSSEReader(resp *http.Response) *bufio.Reader { return bufio.NewReader(resp.Body) }

// fleetStreamFixture serves the multi-bus stream over a real HTTP server.
func fleetStreamFixture(t *testing.T, maxSubs int) (*httptest.Server, *stream.Hub, *movingBus) {
	t.Helper()
	bus := &movingBus{recorded: time.Unix(1_789_836_570, 0).UTC()}
	svc := &mockChaloService{getBusTrackingFunc: bus.get}
	hub := stream.NewHub(bus.get, 20*time.Millisecond, maxSubs)
	srv := httptest.NewServer(newFleetRouter(svc, hub))
	t.Cleanup(func() {
		hub.Close()
		srv.Close()
	})
	return srv, hub, bus
}

func TestFleetStreamMultiplexesBusesOnOneConnection(t *testing.T) {
	srv, hub, bus := fleetStreamFixture(t, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/buses/stream?codes=A1,B2,GONE1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	r := newSSEReader(resp)

	if ev := nextNamed(t, r); ev.name != "retry" {
		t.Fatalf("first frame = %+v", ev)
	}
	positions := map[string]bool{}
	var busErr busErrorEvent
	for i := 0; i < 3; i++ {
		ev := nextNamed(t, r)
		switch ev.name {
		case eventPosition:
			var info models.BusTrackingInfo
			_ = json.Unmarshal([]byte(ev.data), &info)
			positions[info.BusCode] = true
		case eventBusError:
			_ = json.Unmarshal([]byte(ev.data), &busErr)
		default:
			t.Fatalf("unexpected event %+v", ev)
		}
	}
	if !positions["A1"] || !positions["B2"] {
		t.Errorf("initial positions = %v, want A1 and B2", positions)
	}
	if busErr.BusCode != "GONE1" || busErr.Code != "BUS_NOT_FOUND" {
		t.Errorf("bus_error = %+v, want GONE1 BUS_NOT_FOUND", busErr)
	}
	if topics, subs := hub.Stats(); topics != 3 || subs != 3 {
		t.Errorf("hub = %d topics, %d subs; want 3, 3", topics, subs)
	}

	// Movement on the shared feed arrives for each bus, tagged by bus_code.
	bus.advance()
	updated := map[string]bool{}
	for len(updated) < 2 {
		ev := nextNamed(t, r)
		if ev.name == eventPosition {
			var info models.BusTrackingInfo
			_ = json.Unmarshal([]byte(ev.data), &info)
			updated[info.BusCode] = true
		}
	}

	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if topics, subs := hub.Stats(); topics == 0 && subs == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	topics, subs := hub.Stats()
	t.Errorf("after disconnect: %d topics, %d subs; want all released", topics, subs)
}

func TestFleetStreamCapacityIsAllOrNothing(t *testing.T) {
	srv, hub, _ := fleetStreamFixture(t, 2)
	resp, err := http.Get(srv.URL + "/api/v1/buses/stream?codes=A1,B2,C3")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env fleetEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if resp.StatusCode != http.StatusServiceUnavailable || env.Error.Code != "STREAM_CAPACITY" {
		t.Errorf("over capacity: %d %s", resp.StatusCode, env.Error.Code)
	}
	if _, subs := hub.Stats(); subs != 0 {
		t.Errorf("rejected stream left %d subscriptions behind", subs)
	}
}

func TestFleetStreamEndsWhenHubCloses(t *testing.T) {
	srv, hub, _ := fleetStreamFixture(t, 10)
	resp, err := http.Get(srv.URL + "/api/v1/buses/stream?codes=A1,B2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := newSSEReader(resp)
	nextNamed(t, r)
	nextNamed(t, r)

	hub.Close()
	done := make(chan struct{})
	go func() {
		for {
			if _, err := readEvent(t, r); err != nil {
				close(done)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("multi-bus stream did not end after hub shutdown")
	}
}

// --- trail export ---

func historyRouter(points []models.PositionFix) *gin.Engine {
	svc := &mockChaloService{
		getBusHistoryFunc: func(ctx context.Context, code string) (*models.BusHistory, error) {
			return &models.BusHistory{BusCode: code, Points: points, DistanceMeters: 1112}, nil
		},
	}
	return setupTestRouter(svc, nil)
}

var exportPoints = []models.PositionFix{
	{Latitude: 10, Longitude: 76, Speed: 20, RecordedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)},
	{Latitude: 10.01, Longitude: 76, Speed: 25, RecordedAt: time.Date(2026, 11, 19, 10, 0, 30, 0, time.UTC)},
}

func TestHistoryGeoJSONExport(t *testing.T) {
	w := httptest.NewRecorder()
	historyRouter(exportPoints).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bus/KS602/history?format=geojson", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="KS602-trail.geojson"` {
		t.Errorf("Content-Disposition = %q", cd)
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &fc); err != nil {
		t.Fatal(err)
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) != 3 {
		t.Fatalf("got %s with %d features, want FeatureCollection with line + 2 points", fc.Type, len(fc.Features))
	}
	var line [][2]float64
	if err := json.Unmarshal(fc.Features[0].Geometry.Coordinates, &line); err != nil || fc.Features[0].Geometry.Type != "LineString" {
		t.Fatalf("first feature = %s %s", fc.Features[0].Geometry.Type, fc.Features[0].Geometry.Coordinates)
	}
	// RFC 7946 positions are [longitude, latitude] — easy to get backwards.
	if line[1] != [2]float64{76, 10.01} {
		t.Errorf("line[1] = %v, want [76 10.01] (lon, lat)", line[1])
	}
	if fc.Features[2].Properties["recorded_at"] != "2026-11-19T10:00:30Z" || fc.Features[2].Properties["speed"] != 25.0 {
		t.Errorf("point properties = %v", fc.Features[2].Properties)
	}
}

func TestHistoryGeoJSONSinglePointHasNoLineString(t *testing.T) {
	w := httptest.NewRecorder()
	historyRouter(exportPoints[:1]).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bus/KS602/history?format=geojson", nil))
	if strings.Contains(w.Body.String(), "LineString") {
		t.Error("a LineString needs at least two positions (RFC 7946 §3.1.4)")
	}
}

func TestHistoryGPXExport(t *testing.T) {
	w := httptest.NewRecorder()
	historyRouter(exportPoints).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bus/KS602/history?format=gpx", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/gpx+xml" {
		t.Fatalf("status %d, Content-Type %q", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(w.Body.String(), "<?xml") {
		t.Error("GPX must start with an XML declaration")
	}
	var doc gpxDoc
	if err := xml.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("GPX is not well-formed XML: %v", err)
	}
	if doc.Version != "1.1" || doc.Xmlns != "http://www.topografix.com/GPX/1/1" || doc.Track.Name != "KS602" {
		t.Errorf("header = %+v", doc)
	}
	pts := doc.Track.Segment.Points
	if len(pts) != 2 || pts[1].Lat != 10.01 || pts[1].Lon != 76 || pts[1].Time != "2026-11-19T10:00:30Z" {
		t.Errorf("track points = %+v", pts)
	}
}

func TestHistoryRejectsUnknownFormatBeforeUpstreamWork(t *testing.T) {
	called := false
	svc := &mockChaloService{
		getBusHistoryFunc: func(ctx context.Context, code string) (*models.BusHistory, error) {
			called = true
			return &models.BusHistory{BusCode: code}, nil
		},
	}
	w := httptest.NewRecorder()
	setupTestRouter(svc, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bus/KS602/history?format=kml", nil))
	var resp models.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if w.Code != http.StatusBadRequest || resp.Error.Code != "INVALID_FORMAT" {
		t.Errorf("format=kml: %d %s, want 400 INVALID_FORMAT", w.Code, resp.Error.Code)
	}
	if called {
		t.Error("an invalid format must be rejected before any upstream lookup")
	}
}
