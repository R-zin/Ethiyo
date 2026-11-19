package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/stream"
	"github.com/gin-gonic/gin"
)

// eventBusError carries a per-bus error on the multi-bus stream. Unlike the
// single-bus "upstream_error" it names the bus it is about.
const eventBusError = "bus_error"

// fleetFetchConcurrency bounds parallel upstream lookups per fleet request.
// Lookups go through the service's cache and coalescing, so this mostly
// matters for cold caches.
const fleetFetchConcurrency = 6

// busErrorEvent is the payload of a bus_error event.
type busErrorEvent struct {
	BusCode string `json:"bus_code"`
	models.ErrorDetail
}

// FleetHandler serves multi-bus ("My buses") endpoints.
type FleetHandler struct {
	service chalo.Service
	hub     *stream.Hub
	opts    StreamOptions
}

// NewFleetHandler creates a FleetHandler.
func NewFleetHandler(service chalo.Service, hub *stream.Hub, opts StreamOptions) *FleetHandler {
	return &FleetHandler{service: service, hub: hub, opts: opts.withDefaults()}
}

// fetchFleet looks up every bus with bounded concurrency. Results keep the
// request order; a failing bus yields an error item instead of failing all.
func fetchFleet(ctx context.Context, svc chalo.Service, codes []string) []models.FleetItem {
	items := make([]models.FleetItem, len(codes))
	sem := make(chan struct{}, fleetFetchConcurrency)
	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Add(1)
		go func(i int, code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			item := models.FleetItem{BusCode: code}
			info, err := svc.GetBusTracking(ctx, code)
			if err != nil {
				_, errCode, message := classifyError(err)
				item.Error = &models.ErrorDetail{Code: errCode, Message: message}
			} else {
				item.Tracking = info
			}
			items[i] = item
		}(i, code)
	}
	wg.Wait()
	return items
}

// Track returns live positions for several buses in one request.
// GET /api/v1/buses/track?codes=KS602,DL1PC0001
func (h *FleetHandler) Track(c *gin.Context) {
	codes, err := models.ParseBusCodes(c.Query("codes"), models.MaxFleetSize)
	if err != nil {
		HandleError(c, err)
		return
	}
	items := fetchFleet(c.Request.Context(), h.service, codes)
	c.JSON(http.StatusOK, models.NewSuccessResponse(models.FleetResponse{Buses: items}))
}

type taggedEvent struct {
	code string
	ev   stream.Event
}

// Stream pushes position updates for several buses over ONE connection.
// Browsers allow only ~6 concurrent HTTP/1.1 connections per origin, so one
// EventSource per saved bus would starve the page's other requests.
// GET /api/v1/buses/stream?codes=KS602,DL1PC0001
func (h *FleetHandler) Stream(c *gin.Context) {
	codes, err := models.ParseBusCodes(c.Query("codes"), models.MaxFleetSize)
	if err != nil {
		HandleError(c, err)
		return
	}

	subs, err := h.hub.SubscribeMany(codes)
	if err != nil {
		HandleError(c, err)
		return
	}
	defer func() {
		for _, s := range subs {
			s.Close()
		}
	}()

	ctx := c.Request.Context()
	initial := fetchFleet(ctx, h.service, codes)
	if ctx.Err() != nil {
		return
	}

	sse := startSSE(c, h.opts)
	for _, item := range initial {
		if item.Tracking != nil {
			sse.event(eventPosition, item.Tracking)
		} else {
			sse.event(eventBusError, busErrorEvent{BusCode: item.BusCode, ErrorDetail: *item.Error})
		}
	}
	if !sse.ok {
		return
	}

	// Fan the per-bus subscriptions into one channel. Forwarders stop when
	// the handler returns (done) or when the hub shuts down (sub.C closed).
	merged := make(chan taggedEvent, len(subs))
	done := make(chan struct{})
	defer close(done)
	hubClosed := make(chan struct{})
	var closeOnce sync.Once
	for i, sub := range subs {
		go func(code string, sub *stream.Subscription) {
			for ev := range sub.C {
				select {
				case merged <- taggedEvent{code: code, ev: ev}:
				case <-done:
					return
				}
			}
			closeOnce.Do(func() { close(hubClosed) })
		}(codes[i], sub)
	}

	heartbeat := time.NewTicker(h.opts.Heartbeat)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(h.opts.MaxLifetime)
	defer lifetime.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lifetime.C:
			return
		case <-hubClosed:
			return
		case <-heartbeat.C:
			if !sse.ping() {
				return
			}
		case t := <-merged:
			var sent bool
			if t.ev.Err != nil {
				_, code, message := classifyError(t.ev.Err)
				sent = sse.event(eventBusError, busErrorEvent{
					BusCode:     t.code,
					ErrorDetail: models.ErrorDetail{Code: code, Message: message},
				})
			} else {
				sent = sse.event(eventPosition, t.ev.Info)
			}
			if !sent {
				return
			}
		}
	}
}
