package chalo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Scraper coordinates headless browser discovery of Chalo API requests.
type Scraper struct {
	browserMgr *browser.Manager
	baseURL    string
	timeout    time.Duration
}

// NewScraper creates a new Chalo Scraper.
func NewScraper(mgr *browser.Manager, baseURL string, timeout time.Duration) *Scraper {
	return &Scraper{
		browserMgr: mgr,
		baseURL:    baseURL,
		timeout:    timeout,
	}
}

// AcquireSession reserves a browser session for the scraper's configured
// timeout. Exported primarily so tests and callers can surface configuration
// defects (e.g. a nil browser manager) as errors instead of panics.
func (s *Scraper) AcquireSession(ctx context.Context) (*browser.ExecSession, error) {
	if s == nil || s.browserMgr == nil {
		return nil, errors.New("chalo scraper has no browser manager configured")
	}
	return s.browserMgr.AcquireSession(ctx, s.timeout)
}

// runWithTimeout executes fn with the given timeout unless ctx is already done.
func (s *Scraper) runWithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(tctx)
}

// discoveryTarget selects which intercepted endpoint completes discovery.
type discoveryTarget int

const (
	// targetRoute waits for the scheduler routedetailslive XHR — the only
	// endpoint Chalo's current public route pages still emit.
	targetRoute discoveryTarget = iota
	// targetTrack waits for the legacy vasudha route-live-info XHR.
	targetTrack
)

// discoveredURLs collects intercepted network request URLs matching the
// track/route patterns, and signals once the target endpoint is observed.
type discoveredURLs struct {
	mu     sync.Mutex
	track  string
	route  string
	done   chan struct{}
	once   sync.Once
	target discoveryTarget
}

func newDiscoveredURLs(target discoveryTarget) *discoveredURLs {
	return &discoveredURLs{done: make(chan struct{}), target: target}
}

// completeLocked reports whether the target endpoint has been captured.
// Callers must hold d.mu.
func (d *discoveredURLs) completeLocked() bool {
	if d.target == targetTrack {
		return d.track != ""
	}
	return d.route != ""
}

// handle records a network event, returning true if discovery is complete.
func (d *discoveredURLs) handle(ev *network.EventRequestWillBeSent) bool {
	reqURL := ev.Request.URL

	d.mu.Lock()
	if d.track == "" && TrackURLRegex.MatchString(reqURL) {
		d.track = reqURL
	}
	if d.route == "" && RouteURLRegex.MatchString(reqURL) {
		d.route = reqURL
	}
	complete := d.completeLocked()
	d.mu.Unlock()

	if complete {
		d.once.Do(func() { close(d.done) })
	}
	return complete
}

// results snapshots the discovered URLs and whether the target was captured.
func (d *discoveredURLs) results() (track, route string, complete bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.track, d.route, d.completeLocked()
}

// discover navigates to the public route page and intercepts network requests
// until the target endpoint is observed. Any other matching endpoint seen on
// the way is returned too, but is not required.
func (s *Scraper) discover(ctx context.Context, busCode string, target discoveryTarget) (trackURL, routeURL string, err error) {
	// A caller-canceled context must surface as context.Canceled, not a
	// wrapped concurrency error, so handlers can map it correctly.
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	session, err := s.AcquireSession(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire browser session: %w", err)
	}
	defer session.Cancel()

	browserCtx := session.Ctx

	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		return "", "", fmt.Errorf("failed to enable browser network tracking: %w", err)
	}

	found := newDiscoveredURLs(target)
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		if req.Type != network.ResourceTypeXHR && req.Type != network.ResourceTypeFetch {
			return
		}
		found.handle(req)
	})

	targetURL := fmt.Sprintf("%s/app/public-route/%s", s.baseURL, busCode)
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	); err != nil {
		if errors.Is(browserCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return "", "", context.Canceled
		}
		if errors.Is(browserCtx.Err(), context.DeadlineExceeded) {
			return "", "", ErrChaloTimeout
		}
		return "", "", fmt.Errorf("browser navigation error: %w", err)
	}

	select {
	case <-found.done:
		t, r, _ := found.results()
		return t, r, nil
	case <-browserCtx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", "", context.Canceled
		}
		t, r, complete := found.results()
		if complete {
			// The target was captured just as the deadline hit.
			return t, r, nil
		}
		if t != "" || r != "" {
			// Partial discovery: the page is live and emitted a Chalo API
			// call, but never the one we need. That is a discovery failure,
			// not a transport timeout.
			slog.Warn("partial route discovery",
				"bus_code", busCode, "track_url", t, "route_url", r)
			return t, r, ErrDiscoveryFailed
		}
		return "", "", ErrChaloTimeout
	}
}

// DiscoverRoutes navigates to Chalo's public route page and intercepts the
// scheduler route details URL. trackURL is only populated if the page also
// happened to emit the legacy route-live-info XHR; it is never required,
// because Chalo's current pages no longer emit it (live tracking uses the
// deterministic chatbot/raw endpoint instead — see Client.LiveTrackingURL).
func (s *Scraper) DiscoverRoutes(ctx context.Context, busCode string) (trackURL string, routeURL string, err error) {
	return s.discover(ctx, busCode, targetRoute)
}

// DiscoverTrackURL navigates to the public route page and returns the legacy
// route-live-info tracking URL. Retained for older Chalo deployments.
func (s *Scraper) DiscoverTrackURL(ctx context.Context, busCode string) (string, error) {
	trackURL, _, err := s.discover(ctx, busCode, targetTrack)
	return trackURL, err
}
