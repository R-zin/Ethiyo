package chalo

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/breaker"
)

func TestServesLastKnownPositionWhenChaloFails(t *testing.T) {
	obs := newRecordingObserver()
	svc, up := newTestService(t, Options{LastKnownTTL: time.Minute, Observer: obs})
	recorded := time.Now().Add(-40 * time.Second)
	up.set(http.StatusAccepted, livePayloadAt(10.5, 76.25, recorded))

	fresh, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil || fresh.Degraded {
		t.Fatalf("healthy upstream: degraded=%v err=%v", fresh != nil && fresh.Degraded, err)
	}

	up.set(http.StatusBadGateway, "upstream down")
	info, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("expected the last known position instead of an error, got %v", err)
	}
	if !info.Degraded {
		t.Error("fallback response must be flagged degraded")
	}
	if info.Live.Latitude != 10.5 || info.Live.Longitude != 76.25 || !info.Live.RecordedAt.Equal(recorded.Truncate(time.Millisecond)) {
		t.Errorf("fallback position = %+v; want the last good fix", info.Live)
	}
	if info.Live.AgeSeconds < 39 {
		t.Errorf("age = %v; must reflect when the fix was recorded, not when it was served", info.Live.AgeSeconds)
	}
	if got := obs.cache["last_known"]; got != [2]int{1, 0} {
		t.Errorf("last_known hits/misses = %v, want [1 0]", got)
	}
}

func TestNoFallbackForUnknownBusOrMissingHistory(t *testing.T) {
	svc, up := newTestService(t, Options{LastKnownTTL: time.Minute})

	// Never seen + Chalo down: nothing to fall back to.
	up.set(http.StatusBadGateway, "down")
	if _, err := svc.GetBusTracking(context.Background(), "NEW1"); !errors.Is(err, ErrUpstreamService) {
		t.Errorf("unseen bus during outage: err = %v, want ErrUpstreamService", err)
	}

	// Seen, then Chalo says it is gone: that is an answer, not an outage.
	up.set(http.StatusAccepted, livePayloadAt(10, 76, time.Now()))
	if _, err := svc.GetBusTracking(context.Background(), "KS602"); err != nil {
		t.Fatal(err)
	}
	up.set(http.StatusAccepted, unknownVehicleBody)
	if _, err := svc.GetBusTracking(context.Background(), "KS602"); !errors.Is(err, ErrBusNotFound) {
		t.Errorf("bus gone: err = %v; a last known position must not mask 'not found'", err)
	}
}

func TestNoFallbackForCanceledCaller(t *testing.T) {
	svc, up := newTestService(t, Options{LastKnownTTL: time.Minute})
	if _, err := svc.GetBusTracking(context.Background(), "KS602"); err != nil {
		t.Fatal(err)
	}
	up.set(http.StatusBadGateway, "down")
	up.delay = 200 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := svc.GetBusTracking(ctx, "KS602"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v; a caller that gave up gets its own error, not a fallback", err)
	}
}

func TestFallbackDisabledOrExpired(t *testing.T) {
	svc, up := newTestService(t, Options{}) // LastKnownTTL zero: disabled
	_, _ = svc.GetBusTracking(context.Background(), "KS602")
	up.set(http.StatusBadGateway, "down")
	if _, err := svc.GetBusTracking(context.Background(), "KS602"); !errors.Is(err, ErrUpstreamService) {
		t.Errorf("disabled fallback: err = %v", err)
	}

	svc, up = newTestService(t, Options{LastKnownTTL: 30 * time.Millisecond})
	_, _ = svc.GetBusTracking(context.Background(), "KS602")
	time.Sleep(60 * time.Millisecond)
	up.set(http.StatusBadGateway, "down")
	if _, err := svc.GetBusTracking(context.Background(), "KS602"); !errors.Is(err, ErrUpstreamService) {
		t.Errorf("expired fallback: err = %v; too-old positions must not be served", err)
	}
}

func TestCircuitOpensAndFailsFast(t *testing.T) {
	svc, up := newTestService(t, Options{BreakerThreshold: 2, BreakerCooldown: time.Hour})
	up.set(http.StatusBadGateway, "down")
	up.delay = 50 * time.Millisecond

	for i := 0; i < 2; i++ {
		_, _ = svc.GetBusTracking(context.Background(), "KS602")
	}
	if st := svc.UpstreamStatus(); st.State != breaker.Open {
		t.Fatalf("breaker state = %s after 2 failures, want open", st.State)
	}

	calls := up.Calls()
	start := time.Now()
	_, err := svc.GetBusTracking(context.Background(), "OTHER1")
	if !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("err = %v, want breaker.ErrOpen", err)
	}
	if up.Calls() != calls {
		t.Error("an open circuit must not call Chalo")
	}
	if elapsed := time.Since(start); elapsed > 30*time.Millisecond {
		t.Errorf("open circuit took %v; it should fail fast instead of waiting on a timeout", elapsed)
	}
}

func TestOpenCircuitStillServesLastKnown(t *testing.T) {
	svc, up := newTestService(t, Options{BreakerThreshold: 1, BreakerCooldown: time.Hour, LastKnownTTL: time.Minute})
	_, _ = svc.GetBusTracking(context.Background(), "KS602")
	up.set(http.StatusBadGateway, "down")
	_, _ = svc.GetBusTracking(context.Background(), "KS602") // opens the circuit

	calls := up.Calls()
	info, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil || !info.Degraded {
		t.Fatalf("open circuit: degraded=%v err=%v; want last known position", info != nil && info.Degraded, err)
	}
	if up.Calls() != calls {
		t.Error("serving the fallback must not call Chalo while the circuit is open")
	}
}

func TestUnknownBusesDoNotTripTheBreaker(t *testing.T) {
	svc, up := newTestService(t, Options{BreakerThreshold: 2, BreakerCooldown: time.Hour})
	up.set(http.StatusAccepted, unknownVehicleBody)
	for i := 0; i < 10; i++ {
		if _, err := svc.GetBusTracking(context.Background(), "ZZZ1"); !errors.Is(err, ErrBusNotFound) {
			t.Fatalf("err = %v", err)
		}
	}
	if st := svc.UpstreamStatus(); st.State != breaker.Closed {
		t.Errorf("breaker = %s; 'bus not found' is a healthy upstream answer", st.State)
	}
}
