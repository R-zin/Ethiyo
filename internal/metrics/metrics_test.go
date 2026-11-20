package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func render(m *Metrics) string {
	var b strings.Builder
	m.Render(&b)
	return b.String()
}

func newRouter(m *Metrics) *gin.Engine {
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/api/v1/bus/:buscode/track", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", m.Handler())
	return r
}

func TestMiddlewareUsesRouteTemplateToBoundCardinality(t *testing.T) {
	m := New()
	r := newRouter(m)
	for _, code := range []string{"KS602", "DL1PC0001", "A", "B", "C"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/bus/"+code+"/track", nil))
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/wp-admin/../etc/passwd", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PROPFIND", "/x", nil))

	out := render(m)
	want := []string{
		`ethiyo_http_requests_total{method="GET",route="/api/v1/bus/:buscode/track",status="200"} 5`,
		`ethiyo_http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`ethiyo_http_requests_total{method="OTHER",route="unmatched",status="404"} 1`,
		`ethiyo_http_request_duration_seconds_count{route="/api/v1/bus/:buscode/track"} 5`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
	for _, leak := range []string{"KS602", "passwd", "PROPFIND"} {
		if strings.Contains(out, leak) {
			t.Errorf("raw request data %q leaked into labels", leak)
		}
	}
}

func TestHistogramBucketsAreCumulative(t *testing.T) {
	m := New()
	m.ObserveUpstream("live_tracking", 3*time.Millisecond, nil)         // le 0.005
	m.ObserveUpstream("live_tracking", 200*time.Millisecond, nil)       // le 0.25
	m.ObserveUpstream("live_tracking", 30*time.Second, errors.New("x")) // +Inf only

	out := render(m)
	for _, w := range []string{
		`ethiyo_upstream_duration_seconds_bucket{op="live_tracking",le="0.005"} 1`,
		`ethiyo_upstream_duration_seconds_bucket{op="live_tracking",le="0.1"} 1`,
		`ethiyo_upstream_duration_seconds_bucket{op="live_tracking",le="0.25"} 2`,
		`ethiyo_upstream_duration_seconds_bucket{op="live_tracking",le="20"} 2`,
		`ethiyo_upstream_duration_seconds_bucket{op="live_tracking",le="+Inf"} 3`,
		`ethiyo_upstream_duration_seconds_count{op="live_tracking"} 3`,
		`ethiyo_upstream_requests_total{op="live_tracking",outcome="success"} 2`,
		`ethiyo_upstream_requests_total{op="live_tracking",outcome="error"} 1`,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q", w)
		}
	}
	if !strings.Contains(out, `ethiyo_upstream_duration_seconds_sum{op="live_tracking"} 30.203`) {
		t.Errorf("histogram sum wrong:\n%s", out)
	}
}

func TestCacheCountersAndGauges(t *testing.T) {
	m := New()
	m.ObserveCache("live", true)
	m.ObserveCache("live", true)
	m.ObserveCache("live", false)
	m.RegisterGauge("ethiyo_stream_subscribers", "Active subscribers.", func() float64 { return 7 })

	out := render(m)
	for _, w := range []string{
		`ethiyo_cache_requests_total{cache="live",result="hit"} 2`,
		`ethiyo_cache_requests_total{cache="live",result="miss"} 1`,
		"# TYPE ethiyo_stream_subscribers gauge",
		"ethiyo_stream_subscribers 7",
		"# TYPE go_goroutines gauge",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q", w)
		}
	}
}

// TestExpositionFormatIsWellFormed checks every line against the Prometheus
// text format grammar, including hostile label values.
func TestExpositionFormatIsWellFormed(t *testing.T) {
	m := New()
	m.ObserveCache("we\"ird\\name\nline\ttabé", true)
	m.ObserveUpstream("op", time.Millisecond, nil)
	r := newRouter(m)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/bus/X/track", nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Errorf("Content-Type = %q", ct)
	}

	labelValue := `"(?:[^"\\\n]|\\["\\n])*"`
	sample := regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[a-zA-Z_][a-zA-Z0-9_]*=` + labelValue +
		`(,[a-zA-Z_][a-zA-Z0-9_]*=` + labelValue + `)*\})? [-+0-9.eE]+|\+Inf$`)
	meta := regexp.MustCompile(`^# (HELP|TYPE) [a-zA-Z_:][a-zA-Z0-9_:]* .+$`)

	for i, line := range strings.Split(strings.TrimSuffix(w.Body.String(), "\n"), "\n") {
		if !sample.MatchString(line) && !meta.MatchString(line) {
			t.Errorf("line %d is not valid exposition format: %q", i+1, line)
		}
	}
}
