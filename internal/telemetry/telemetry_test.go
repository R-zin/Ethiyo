package telemetry

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f ± %.4f", name, got, want, tol)
	}
}

func TestHaversineKnownDistances(t *testing.T) {
	approx(t, "same point", HaversineMeters(12.97, 77.59, 12.97, 77.59), 0, 1e-9)
	// One degree of latitude ≈ 111.195 km on the mean sphere.
	approx(t, "1° latitude", HaversineMeters(0, 0, 1, 0), 111195, 5)
	// Bengaluru → Chennai ≈ 290 km great-circle.
	approx(t, "BLR→MAA", HaversineMeters(12.9716, 77.5946, 13.0827, 80.2707), 290_000, 3_000)
	// Antipodal points must not produce NaN.
	if d := HaversineMeters(0, 0, 0, 180); math.IsNaN(d) || d < 20_000_000 {
		t.Errorf("antipodal distance = %v", d)
	}
}

func TestBearingCardinalDirections(t *testing.T) {
	cases := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		want                   float64
	}{
		{"north", 10, 76, 10.01, 76, 0},
		{"east", 10, 76, 10, 76.01, 90},
		{"south", 10, 76, 9.99, 76, 180},
		{"west", 10, 76, 10, 75.99, 270},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BearingDegrees(c.lat1, c.lon1, c.lat2, c.lon2)
			// Bearing to due west is reported as 270, not -90.
			approx(t, "bearing", got, c.want, 0.05)
			if got < 0 || got >= 360 {
				t.Errorf("bearing %v out of [0,360)", got)
			}
		})
	}
}

func fixAt(lat, lon float64, sec int64) models.PositionFix {
	return models.PositionFix{Latitude: lat, Longitude: lon, Speed: 20, RecordedAt: time.Unix(sec, 0)}
}

func TestRecordDerivesHeadingAndIgnoresJitter(t *testing.T) {
	h := NewHistory(10, 10)
	if hd := h.Record("B", fixAt(10, 76, 100)); hd != nil {
		t.Fatalf("first fix must have no heading, got %v", *hd)
	}
	hd := h.Record("B", fixAt(10.01, 76, 130)) // ~1.1 km north
	if hd == nil {
		t.Fatal("expected heading after movement")
	}
	approx(t, "heading", *hd, 0, 0.05)

	// A ~1 m wiggle at a stop is jitter: the northbound heading must persist.
	hd = h.Record("B", fixAt(10.01001, 76.00001, 160))
	if hd == nil {
		t.Fatal("heading lost on jitter")
	}
	approx(t, "heading after jitter", *hd, 0, 0.05)

	// Turning east updates the heading.
	hd = h.Record("B", fixAt(10.01001, 76.01, 190))
	approx(t, "heading after turn", *hd, 90, 0.1)
}

func TestRecordDeduplicatesAndRejectsOutOfOrder(t *testing.T) {
	h := NewHistory(10, 10)
	h.Record("B", fixAt(10, 76, 100))
	h.Record("B", fixAt(10, 76, 100))    // duplicate (e.g. cached response)
	h.Record("B", fixAt(10.5, 76.5, 50)) // older than the last fix
	pts, _ := h.Trail("B")
	if len(pts) != 1 {
		t.Fatalf("trail len = %d, want 1 (duplicate and stale fixes ignored)", len(pts))
	}
}

func TestTrailIsBoundedAndOrdered(t *testing.T) {
	h := NewHistory(3, 10)
	for i := int64(0); i < 5; i++ {
		h.Record("B", fixAt(10+float64(i)*0.01, 76, 100+i))
	}
	pts, dist := h.Trail("B")
	if len(pts) != 3 {
		t.Fatalf("trail len = %d, want capacity 3", len(pts))
	}
	for i, want := range []int64{102, 103, 104} {
		if pts[i].RecordedAt.Unix() != want {
			t.Errorf("pts[%d] at %d, want %d (oldest first, oldest dropped)", i, pts[i].RecordedAt.Unix(), want)
		}
	}
	approx(t, "distance", dist, 2*HaversineMeters(10, 76, 10.01, 76), 1)

	// Trail returns a copy: mutating it must not corrupt history.
	pts[0].Latitude = -1
	again, _ := h.Trail("B")
	if again[0].Latitude == -1 {
		t.Error("Trail must return a defensive copy")
	}
}

func TestTrailUnknownBusIsEmptyNotNil(t *testing.T) {
	pts, dist := NewHistory(0, 0).Trail("nope")
	if pts == nil || len(pts) != 0 || dist != 0 {
		t.Errorf("Trail(unknown) = %v,%v; want empty non-nil slice (serializes as [])", pts, dist)
	}
}

func TestMaxBusesEvictsLeastRecentlySeen(t *testing.T) {
	h := NewHistory(5, 2)
	clock := time.Unix(1_000, 0)
	h.now = func() time.Time { return clock }

	h.Record("A", fixAt(10, 76, 1))
	clock = clock.Add(time.Second)
	h.Record("B", fixAt(10, 76, 1))
	clock = clock.Add(time.Second)
	h.Record("A", fixAt(10, 76, 2)) // A is now more recent than B
	clock = clock.Add(time.Second)
	h.Record("C", fixAt(10, 76, 1)) // evicts B

	if h.Len() != 2 {
		t.Fatalf("Len = %d, want 2", h.Len())
	}
	if pts, _ := h.Trail("B"); len(pts) != 0 {
		t.Error("least recently seen bus B should have been evicted")
	}
	if pts, _ := h.Trail("A"); len(pts) == 0 {
		t.Error("recently seen bus A must survive eviction")
	}
}

func TestHistoryConcurrentUse(t *testing.T) {
	h := NewHistory(20, 50)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				code := string(rune('A' + (i % 5)))
				h.Record(code, fixAt(10+float64(i)*1e-3, 76, int64(g*1000+i)))
				h.Trail(code)
				h.Heading(code)
			}
		}(g)
	}
	wg.Wait()
}
