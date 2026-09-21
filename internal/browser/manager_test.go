package browser

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFindBrowserExecutableWithNonExistentPath(t *testing.T) {
	_, err := FindBrowserExecutable("/path/that/definitely/does/not/exist/chrome")
	if err == nil {
		t.Fatalf("expected error for non-existent path")
	}
}

func TestFindBrowserExecutableWithCustomFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "fake-browser-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	path, err := FindBrowserExecutable(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != tmpFile.Name() {
		t.Errorf("expected %q, got %q", tmpFile.Name(), path)
	}
}

func TestFindBrowserExecutableDiscovery(t *testing.T) {
	// Discover browser on current machine (non-fatal if environment has no GUI/Chrome)
	path, err := FindBrowserExecutable("")
	if err == nil {
		if path == "" {
			t.Errorf("expected non-empty browser path when err is nil")
		}
		t.Logf("discovered browser at: %s", path)
	} else {
		t.Logf("no browser discovered in test environment: %v (expected if no browser installed)", err)
	}
}

func TestManagerConcurrencyLimit(t *testing.T) {
	mgr := NewManager("", true, 5*time.Second, 2)

	// Acquire first session
	sess1, err := mgr.AcquireSession(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error acquiring sess1: %v", err)
	}
	defer sess1.Cancel()

	// Acquire second session
	sess2, err := mgr.AcquireSession(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error acquiring sess2: %v", err)
	}
	defer sess2.Cancel()

	// Third acquire should block; test with short timeout context
	ctxShort, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()

	_, err = mgr.AcquireSession(ctxShort, 5*time.Second)
	if err == nil {
		t.Fatalf("expected error when acquiring session beyond concurrency limit")
	}

	// Release one session and try again
	sess2.Cancel()

	ctxAvailable, cancelAvailable := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelAvailable()

	sess3, err := mgr.AcquireSession(ctxAvailable, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error acquiring sess3 after release: %v", err)
	}
	sess3.Cancel()
}

func TestManagerRejectsInvalidTimeout(t *testing.T) {
	mgr := NewManager("", true, 5*time.Second, 1)
	_, err := mgr.AcquireSession(context.Background(), 0)
	if !errors.Is(err, ErrInvalidTimeout) {
		t.Fatalf("expected ErrInvalidTimeout for zero timeout, got %v", err)
	}
	_, err = mgr.AcquireSession(context.Background(), -time.Second)
	if !errors.Is(err, ErrInvalidTimeout) {
		t.Fatalf("expected ErrInvalidTimeout for negative timeout, got %v", err)
	}
}

func TestAcquireSessionCallerContextCanceled(t *testing.T) {
	mgr := NewManager("", true, 5*time.Second, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mgr.AcquireSession(ctx, time.Second)
	if err == nil {
		t.Fatalf("expected error acquiring with canceled context")
	}
}

// TestSetExecPathConcurrent exercises the data race between AcquireSession
// (which reads execPath) and SetExecPath (which writes it) under -race.
func TestSetExecPathConcurrent(t *testing.T) {
	mgr := NewManager("", true, 5*time.Second, 4)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mgr.SetExecPath("/tmp/chrome")
		}()
		go func() {
			defer wg.Done()
			sess, err := mgr.AcquireSession(context.Background(), 50*time.Millisecond)
			if err == nil {
				sess.Cancel()
			}
		}()
	}
	wg.Wait()
}

func TestAcquireAndReleaseCycle(t *testing.T) {
	mgr := NewManager("", true, 5*time.Second, 1)
	sess, err := mgr.AcquireSession(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	sess.Cancel()

	// Slot must be immediately re-acquirable after release.
	sess2, err := mgr.AcquireSession(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	sess2.Cancel()
}
