package chalo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/browser"
	"github.com/chromedp/chromedp"
)

// requireBrowser skips the test when no Chromium-based browser is available.
func requireBrowser(t *testing.T) string {
	t.Helper()
	path, err := browser.FindBrowserExecutable("")
	if err != nil {
		t.Skipf("no browser available: %v", err)
	}
	return path
}

// newDiscoveryFixture serves a page that fires XHRs to both a tracking and a
// route-details URL (via fetch, expected to fail CORS — the request is still
// observable through the network domain), plus a route-details JSON API.
func newDiscoveryFixture(t *testing.T) (scraper *Scraper, apiHits *int32, cleanup func()) {
	t.Helper()

	browserPath := requireBrowser(t)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	trackURL := server.URL + "/app/api/vasudha/track/route-live-info/X"
	routeURL := server.URL + "/app/api/scheduler_v4/v4/testcity/routedetailslive?routeId=X"

	mux.HandleFunc("/app/public-route/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head></head><body>
			<script>
				fetch(%q).catch(function(){});
				fetch(%q).catch(function(){});
			</script>
			hello</body></html>`, trackURL, routeURL)
	})
	mux.HandleFunc("/app/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})

	mgr := browser.NewManager(browserPath, true, 10*time.Second, 2)
	scraper = NewScraper(mgr, server.URL, 10*time.Second)

	return scraper, nil, func() {
		server.Close()
	}
}

func TestDiscoverRoutesHitsConfiguredBaseURL(t *testing.T) {
	scraper, _, cleanup := newDiscoveryFixture(t)
	defer cleanup()

	// Discovered URLs point at the test server (not chalo.com), so a match
	// proves discovery honored the configured base URL and that the regexes
	// now match scheme-less URLs — required when CHALO_BASE_URL is not
	// https://chalo.com (staging proxies, local dev, tests).
	trackURL, routeURL, err := scraper.DiscoverRoutes(context.Background(), "X")
	if err != nil {
		t.Fatalf("DiscoverRoutes: %v", err)
	}
	if !IsTrackURL(trackURL) {
		t.Errorf("discovered trackURL %q does not match IsTrackURL", trackURL)
	}
	if !IsRouteURL(routeURL) {
		t.Errorf("discovered routeURL %q does not match IsRouteURL", routeURL)
	}
}

func TestDiscoverTrackURLHitsConfiguredBaseURL(t *testing.T) {
	scraper, _, cleanup := newDiscoveryFixture(t)
	defer cleanup()

	trackURL, err := scraper.DiscoverTrackURL(context.Background(), "X")
	if err != nil {
		t.Fatalf("DiscoverTrackURL: %v", err)
	}
	if trackURL == "" {
		t.Fatalf("expected non-empty track URL")
	}
	if !IsTrackURL(trackURL) {
		t.Errorf("discovered trackURL %q does not match IsTrackURL", trackURL)
	}
}

func TestDiscoverTrackURLRespectsCallerCancellation(t *testing.T) {
	scraper, _, cleanup := newDiscoveryFixture(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the call

	start := time.Now()
	_, err := scraper.DiscoverTrackURL(ctx, "X")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("cancellation took too long: %v", time.Since(start))
	}
}

func TestSharedTargetDiscoveryCompletes(t *testing.T) {
	// Regression: on a busy manager, DiscoverRoutes and DiscoverTrackURL for
	// the same page share a browser target via single-flight; a hung or
	// duplicated listener on the shared context would block both callers.
	scraper, _, cleanup := newDiscoveryFixture(t)
	defer cleanup()

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 2; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, _, err := scraper.DiscoverRoutes(context.Background(), "X"); err != nil {
				errs <- fmt.Errorf("DiscoverRoutes: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := scraper.DiscoverTrackURL(context.Background(), "X"); err != nil {
				errs <- fmt.Errorf("DiscoverTrackURL: %w", err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("%v", err)
	}
}

func TestAcquireSessionRejectsNilManager(t *testing.T) {
	var s *Scraper // nil browser manager (construction defect)
	_, err := s.AcquireSession(context.Background())
	if err == nil {
		t.Fatalf("expected error for nil browser manager")
	}
}

// TestRunWithTimeoutGuardsBrowser ensures the internal timeout wrapper honors
// an already-canceled parent context.
func TestRunWithTimeoutGuardsBrowser(t *testing.T) {
	var s *Scraper
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.runWithTimeout(ctx, time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx)
	})
	if err == nil {
		t.Fatalf("expected error on canceled context")
	}
}
