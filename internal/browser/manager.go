package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	// ErrConcurrencyLimitReached is returned if the browser worker pool is saturated and context times out.
	ErrConcurrencyLimitReached = errors.New("browser concurrency limit reached, operation timed out waiting for worker")

	// ErrInvalidTimeout is returned when a non-positive timeout is supplied for an operation
	// while the manager has no usable default configured.
	ErrInvalidTimeout = errors.New("timeout must be positive")
)

// Manager coordinates chromedp browser execution and enforces concurrency limits.
type Manager struct {
	mu             sync.RWMutex // guards execPath
	execPath       string
	headless       bool
	defaultTimeout time.Duration
	sem            chan struct{}
}

// NewManager creates a new BrowserManager.
func NewManager(execPath string, headless bool, defaultTimeout time.Duration, maxConcurrency int) *Manager {
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	return &Manager{
		execPath:       execPath,
		headless:       headless,
		defaultTimeout: defaultTimeout,
		sem:            make(chan struct{}, maxConcurrency),
	}
}

// ExecSession holds the chromedp context and cleanup function for a browser operation.
type ExecSession struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// AcquireSession acquires a concurrency slot and initializes a chromedp context tied to parentCtx.
// The caller MUST call session.Cancel() when done to release resources and the concurrency slot.
func (m *Manager) AcquireSession(parentCtx context.Context, timeout time.Duration) (*ExecSession, error) {
	if timeout <= 0 {
		return nil, ErrInvalidTimeout
	}

	// Fast path: an already-canceled caller context fails immediately rather
	// than racing a free semaphore slot in the select below.
	if err := parentCtx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConcurrencyLimitReached, err)
	}

	// 1. Acquire concurrency token
	select {
	case m.sem <- struct{}{}:
	case <-parentCtx.Done():
		return nil, fmt.Errorf("%w: %v", ErrConcurrencyLimitReached, parentCtx.Err())
	}

	// 2. Prepare chromedp options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", m.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("mute-audio", true),
	)

	if p := m.ExecPath(); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parentCtx, opts...)
	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, timeout)

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancelTimeout()
			cancelTask()
			cancelAlloc()
			// Release concurrency token
			select {
			case <-m.sem:
			default:
			}
		})
	}

	return &ExecSession{
		Ctx:    timeoutCtx,
		Cancel: cleanup,
	}, nil
}

// ExecPath returns the configured browser executable path.
func (m *Manager) ExecPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.execPath
}

// SetExecPath updates the executable path (e.g. after dynamic discovery).
func (m *Manager) SetExecPath(path string) {
	m.mu.Lock()
	m.execPath = path
	m.mu.Unlock()
	slog.Info("browser executable configured", "path", path)
}
