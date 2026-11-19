package telemetry

import (
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

const (
	// DefaultPointsPerBus is how many recent fixes are retained per bus. At a
	// typical 30s telemetry cadence this covers roughly half an hour.
	DefaultPointsPerBus = 60
	// DefaultMaxBuses bounds how many buses are tracked at once.
	DefaultMaxBuses = 1000
	// MinMovementMeters is the displacement below which consecutive fixes are
	// treated as GPS jitter of a stationary bus, so the last known heading is
	// kept rather than recomputed from noise.
	MinMovementMeters = 10.0
)

type track struct {
	fixes    []models.PositionFix // oldest first, len <= capacity
	heading  *float64
	lastSeen time.Time
}

// History keeps a bounded ring buffer of recent fixes for each bus and
// derives the direction of travel. It is safe for concurrent use.
type History struct {
	mu       sync.Mutex
	perBus   int
	maxBuses int
	buses    map[string]*track
	now      func() time.Time
}

// NewHistory creates a History. Non-positive arguments use the defaults.
func NewHistory(pointsPerBus, maxBuses int) *History {
	if pointsPerBus <= 0 {
		pointsPerBus = DefaultPointsPerBus
	}
	if maxBuses <= 0 {
		maxBuses = DefaultMaxBuses
	}
	return &History{
		perBus:   pointsPerBus,
		maxBuses: maxBuses,
		buses:    make(map[string]*track),
		now:      time.Now,
	}
}

// Record adds a fix for busCode and returns the bus's current heading (nil if
// it has not yet been seen moving). Fixes that are not newer than the last
// recorded fix (duplicates from cached or repeated upstream responses, or
// out-of-order deliveries) are ignored.
func (h *History) Record(busCode string, fix models.PositionFix) *float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	t, ok := h.buses[busCode]
	if !ok {
		if len(h.buses) >= h.maxBuses {
			h.evictOldestLocked()
		}
		t = &track{fixes: make([]models.PositionFix, 0, h.perBus)}
		h.buses[busCode] = t
	}
	t.lastSeen = h.now()

	if n := len(t.fixes); n > 0 {
		last := t.fixes[n-1]
		if !fix.RecordedAt.After(last.RecordedAt) {
			return copyHeading(t.heading)
		}
		if HaversineMeters(last.Latitude, last.Longitude, fix.Latitude, fix.Longitude) >= MinMovementMeters {
			hd := BearingDegrees(last.Latitude, last.Longitude, fix.Latitude, fix.Longitude)
			t.heading = &hd
		}
	}

	if len(t.fixes) == h.perBus {
		copy(t.fixes, t.fixes[1:])
		t.fixes = t.fixes[:h.perBus-1]
	}
	t.fixes = append(t.fixes, fix)
	return copyHeading(t.heading)
}

// Heading returns the last known heading for busCode.
func (h *History) Heading(busCode string) *float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.buses[busCode]; ok {
		return copyHeading(t.heading)
	}
	return nil
}

// Trail returns a copy of the recorded fixes (oldest first) and the total
// distance travelled along them, in meters.
func (h *History) Trail(busCode string) ([]models.PositionFix, float64) {
	h.mu.Lock()
	t, ok := h.buses[busCode]
	var fixes []models.PositionFix
	if ok {
		fixes = make([]models.PositionFix, len(t.fixes))
		copy(fixes, t.fixes)
	}
	h.mu.Unlock()

	if fixes == nil {
		return []models.PositionFix{}, 0
	}
	var dist float64
	for i := 1; i < len(fixes); i++ {
		dist += HaversineMeters(fixes[i-1].Latitude, fixes[i-1].Longitude, fixes[i].Latitude, fixes[i].Longitude)
	}
	return fixes, dist
}

// Len returns the number of buses currently tracked.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.buses)
}

func (h *History) evictOldestLocked() {
	var (
		oldestKey  string
		oldestSeen time.Time
		found      bool
	)
	for k, t := range h.buses {
		if !found || t.lastSeen.Before(oldestSeen) {
			oldestKey, oldestSeen, found = k, t.lastSeen, true
		}
	}
	if found {
		delete(h.buses, oldestKey)
	}
}

func copyHeading(h *float64) *float64 {
	if h == nil {
		return nil
	}
	v := *h
	return &v
}
