// Package metrics is a small, dependency-free Prometheus exposition of the
// signals that matter for Ethiyo: HTTP traffic, upstream Chalo health, cache
// effectiveness, live streams and browser pool pressure.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// latencyBuckets (seconds) span fast cached responses up to slow browser
// discovery.
var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20}

type histogram struct {
	counts []uint64 // cumulative is computed at render time
	sum    float64
	count  uint64
}

func newHistogram() *histogram { return &histogram{counts: make([]uint64, len(latencyBuckets))} }

func (h *histogram) observe(v float64) {
	for i, b := range latencyBuckets {
		if v <= b {
			h.counts[i]++
			break
		}
	}
	h.sum += v
	h.count++
}

type gauge struct {
	name, help string
	fn         func() float64
}

// Metrics collects and renders application metrics. It is safe for
// concurrent use.
type Metrics struct {
	mu        sync.Mutex
	requests  map[[3]string]uint64 // method, route, status
	durations map[string]*histogram
	upstream  map[[2]string]uint64 // op, outcome
	upDur     map[string]*histogram
	cache     map[[2]string]uint64 // cache, result
	gauges    []gauge
	started   time.Time
}

// New creates an empty Metrics with process gauges pre-registered.
func New() *Metrics {
	m := &Metrics{
		requests:  make(map[[3]string]uint64),
		durations: make(map[string]*histogram),
		upstream:  make(map[[2]string]uint64),
		upDur:     make(map[string]*histogram),
		cache:     make(map[[2]string]uint64),
		started:   time.Now(),
	}
	m.RegisterGauge("ethiyo_uptime_seconds", "Seconds since the process started.", func() float64 {
		return time.Since(m.started).Seconds()
	})
	m.RegisterGauge("go_goroutines", "Number of goroutines that currently exist.", func() float64 {
		return float64(runtime.NumGoroutine())
	})
	return m
}

// RegisterGauge adds a gauge whose value is sampled at scrape time.
func (m *Metrics) RegisterGauge(name, help string, fn func() float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges = append(m.gauges, gauge{name: name, help: help, fn: fn})
}

var knownMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodOptions: true,
}

// Middleware records request counts and latency per route template. Using
// the template (not the raw path) keeps label cardinality bounded no matter
// how many bus codes or junk URLs clients request.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		if !knownMethods[method] {
			method = "OTHER"
		}
		status := strconv.Itoa(c.Writer.Status())

		m.mu.Lock()
		m.requests[[3]string{method, route, status}]++
		h, ok := m.durations[route]
		if !ok {
			h = newHistogram()
			m.durations[route] = h
		}
		h.observe(time.Since(start).Seconds())
		m.mu.Unlock()
	}
}

// ObserveCache implements chalo.Observer.
func (m *Metrics) ObserveCache(name string, hit bool) {
	result := "miss"
	if hit {
		result = "hit"
	}
	m.mu.Lock()
	m.cache[[2]string{name, result}]++
	m.mu.Unlock()
}

// ObserveUpstream implements chalo.Observer.
func (m *Metrics) ObserveUpstream(op string, d time.Duration, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	m.mu.Lock()
	m.upstream[[2]string{op, outcome}]++
	h, ok := m.upDur[op]
	if !ok {
		h = newHistogram()
		m.upDur[op] = h
	}
	h.observe(d.Seconds())
	m.mu.Unlock()
}

// Handler serves the metrics in Prometheus text exposition format 0.0.4.
func (m *Metrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.Status(http.StatusOK)
		m.Render(c.Writer)
	}
}

// Render writes all metrics in a deterministic order.
func (m *Metrics) Render(w io.Writer) {
	m.mu.Lock()
	gauges := append([]gauge(nil), m.gauges...)
	var b strings.Builder

	writeHeader(&b, "ethiyo_http_requests_total", "counter", "HTTP requests by method, route template and status.")
	for _, k := range sortedKeys3(m.requests) {
		fmt.Fprintf(&b, "ethiyo_http_requests_total{method=%q,route=%q,status=%q} %d\n",
			escape(k[0]), escape(k[1]), escape(k[2]), m.requests[k])
	}
	writeHistograms(&b, "ethiyo_http_request_duration_seconds", "HTTP request latency by route template.", "route", m.durations)

	writeHeader(&b, "ethiyo_upstream_requests_total", "counter", "Calls to Chalo by operation and outcome.")
	for _, k := range sortedKeys2(m.upstream) {
		fmt.Fprintf(&b, "ethiyo_upstream_requests_total{op=%q,outcome=%q} %d\n", escape(k[0]), escape(k[1]), m.upstream[k])
	}
	writeHistograms(&b, "ethiyo_upstream_duration_seconds", "Chalo call latency by operation.", "op", m.upDur)

	writeHeader(&b, "ethiyo_cache_requests_total", "counter", "Cache lookups by cache and result.")
	for _, k := range sortedKeys2(m.cache) {
		fmt.Fprintf(&b, "ethiyo_cache_requests_total{cache=%q,result=%q} %d\n", escape(k[0]), escape(k[1]), m.cache[k])
	}
	m.mu.Unlock()

	// Sample gauges outside the lock: callbacks may take other locks.
	for _, g := range gauges {
		writeHeader(&b, g.name, "gauge", g.help)
		fmt.Fprintf(&b, "%s %s\n", g.name, formatFloat(g.fn()))
	}

	_, _ = io.WriteString(w, b.String())
}

func writeHeader(b *strings.Builder, name, typ, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
}

func writeHistograms(b *strings.Builder, name, help, label string, hs map[string]*histogram) {
	writeHeader(b, name, "histogram", help)
	keys := make([]string, 0, len(hs))
	for k := range hs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h := hs[k]
		var cum uint64
		for i, le := range latencyBuckets {
			cum += h.counts[i]
			fmt.Fprintf(b, "%s_bucket{%s=%q,le=%q} %d\n", name, label, escape(k), formatFloat(le), cum)
		}
		fmt.Fprintf(b, "%s_bucket{%s=%q,le=\"+Inf\"} %d\n", name, label, escape(k), h.count)
		fmt.Fprintf(b, "%s_sum{%s=%q} %s\n", name, label, escape(k), formatFloat(h.sum))
		fmt.Fprintf(b, "%s_count{%s=%q} %d\n", name, label, escape(k), h.count)
	}
}

// escape prepares a label value for %q-style quoting. Go's %q already
// escapes backslashes, quotes and newlines compatibly with the exposition
// format for the ASCII values used here; non-printable bytes are replaced so
// %q never emits \x or \u sequences Prometheus cannot parse.
func escape(s string) string {
	return strings.Map(func(r rune) rune {
		if (r < 0x20 && r != '\n') || r == 0x7f || r > 0x7e {
			return '_'
		}
		return r
	}, s)
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func sortedKeys3(m map[[3]string]uint64) [][3]string {
	keys := make([][3]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		for x := 0; x < 3; x++ {
			if keys[i][x] != keys[j][x] {
				return keys[i][x] < keys[j][x]
			}
		}
		return false
	})
	return keys
}

func sortedKeys2(m map[[2]string]uint64) [][2]string {
	keys := make([][2]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	return keys
}
