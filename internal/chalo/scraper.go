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

// DiscoverRoutes navigates to Chalo's public route page and intercepts both the live tracking URL
// and the scheduler route details URL.
func (s *Scraper) DiscoverRoutes(ctx context.Context, busCode string) (trackURL string, routeURL string, err error) {
	session, err := s.browserMgr.AcquireSession(ctx, s.timeout)
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire browser session: %w", err)
	}
	defer session.Cancel()

	browserCtx := session.Ctx

	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		return "", "", fmt.Errorf("failed to enable browser network tracking: %w", err)
	}

	done := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		if req.Type != network.ResourceTypeXHR && req.Type != network.ResourceTypeFetch {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		if trackMatch := TrackURLRegex.FindString(req.Request.URL); trackMatch != "" {
			trackURL = trackMatch
		}

		if routeMatch := RouteURLRegex.FindString(req.Request.URL); routeMatch != "" {
			routeURL = req.Request.URL
		}

		if trackURL != "" && routeURL != "" {
			once.Do(func() {
				close(done)
			})
		}
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
	case <-done:
		return trackURL, routeURL, nil
	case <-browserCtx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", "", context.Canceled
		}
		mu.Lock()
		hasPartial := trackURL != "" || routeURL != ""
		mu.Unlock()
		if hasPartial {
			slog.Warn("partial route discovery on timeout", "trackURL", trackURL, "routeURL", routeURL)
			return trackURL, routeURL, nil
		}
		return "", "", ErrChaloTimeout
	}
}

// DiscoverTrackURL navigates to the public route page and returns the live tracking URL only.
func (s *Scraper) DiscoverTrackURL(ctx context.Context, busCode string) (string, error) {
	session, err := s.browserMgr.AcquireSession(ctx, s.timeout)
	if err != nil {
		return "", fmt.Errorf("failed to acquire browser session: %w", err)
	}
	defer session.Cancel()

	browserCtx := session.Ctx

	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		return "", fmt.Errorf("failed to enable browser network tracking: %w", err)
	}

	done := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var trackURL string

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		if req.Type != network.ResourceTypeXHR && req.Type != network.ResourceTypeFetch {
			return
		}

		if match := TrackURLRegex.FindString(req.Request.URL); match != "" {
			mu.Lock()
			trackURL = match
			mu.Unlock()
			once.Do(func() {
				close(done)
			})
		}
	})

	targetURL := fmt.Sprintf("%s/app/public-route/%s", s.baseURL, busCode)
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	); err != nil {
		if errors.Is(browserCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return "", context.Canceled
		}
		if errors.Is(browserCtx.Err(), context.DeadlineExceeded) {
			return "", ErrChaloTimeout
		}
		return "", fmt.Errorf("browser navigation error: %w", err)
	}

	select {
	case <-done:
		return trackURL, nil
	case <-browserCtx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", context.Canceled
		}
		return "", ErrChaloTimeout
	}
}
