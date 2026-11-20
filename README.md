# Ethiyo
[![CI](https://github.com/R-zin/Ethiyo/actions/workflows/actions.yml/badge.svg)](https://github.com/R-zin/Ethiyo/actions/workflows/actions.yml)

A production-grade, resilient Go backend service for Chalo bus route discovery and real-time transit tracking.

Ethiyo bridges client applications and the Chalo public transit platform. It resolves live bus positions directly from Chalo's vehicle-tracking API and uses automated Chromium-based browser orchestration (`chromedp`) with HTTP request interception to discover route schedules, exposing them through clean, versioned RESTful APIs with strict security, rate limiting, and observability.

---

## What is Ethiyo?

Chalo (`chalo.com`) is a major public transit technology platform operating across multiple cities. While Chalo exposes public transit tracking via web interfaces, its backend APIs utilize dynamic tokens, city identifiers, and live session endpoints that are resolved through client-side browser execution.

Ethiyo automates this lifecycle:
1. Accepts a transit bus code (e.g., `KS602`, `DL1PC0001`, `route_500`).
2. Resolves the bus's live GPS position directly from Chalo's vehicle-tracking API (`dashboard/chatbot/raw`) — no browser needed for the hot path.
3. For route schedules, runs a lightweight headless browser worker to navigate to the route's public page and intercepts backend XHR/Fetch requests matching the scheduler endpoint.
4. Returns structured JSON containing the live position snapshot, live tracking URL, and scheduler route details.
5. Manages authentication, concurrency limits, timeouts, and error handling so clients receive deterministic, reliable responses.

---

## Features

- **Real-Time Streaming (SSE)**: `GET /api/v1/bus/:buscode/stream` pushes position updates to browsers as server-sent events. One shared upstream poller per bus serves every viewer, publishing only when the position changes.
- **Motion Intelligence**: Every live response includes the bus's `heading` (derived from successive GPS fixes), `age_seconds` and a `stale` flag. `GET /api/v1/bus/:buscode/history` returns a breadcrumb trail plus distance travelled.
- **Caching with Request Coalescing**: Live positions are cached briefly (5s) and expensive headless-browser route discoveries for longer (10m). Concurrent identical requests share a single upstream call.
- **Prometheus Metrics**: Dependency-free `/metrics` with per-route request rates and latency, upstream Chalo outcomes, cache hit ratio, live-stream counts and browser-pool pressure.
- **"How Far Is My Bus?"**: The web app can show straight-line distance and an approximate ETA from your location. It is opt-in, and your location never leaves the browser.
- **My Buses (Saved Buses)**: Star buses to pin them to the home screen with live status for each. `GET /api/v1/buses/track` fetches up to 20 buses in one request, and `GET /api/v1/buses/stream` keeps them all live over a **single** connection (browsers allow only about 6 connections per site).
- **Arrival and Disruption Alerts**: "Notify me when my bus is ≤ N minutes away" and "notify me if it stops reporting", as browser notifications plus in-page banners. Evaluated on your device; alerts work while the page is open.
- **Graceful Degradation**: A circuit breaker pauses calls to Chalo after repeated failures. While Chalo is down, each bus's **last known position** is served flagged `degraded: true` instead of an error. Chalo's status shows in `/health`, `/metrics` and the app header.
- **Trail Export**: Download a bus's recent path as GeoJSON or GPX (`/history?format=geojson|gpx`).
- **OpenAPI 3.1**: The API is described at `/api/v1/openapi.json`. Tests fail if a route or model field drifts from the document.
- **Installable App (PWA)**: Add to home screen, with an offline app shell. Live data is never cached.
- **Live GPS Tracking**: Fetches real-time bus positions (lat/lon, speed, next/previous stop, route name) directly from Chalo's `dashboard/chatbot/raw?vehicleNo=` endpoint — no headless browser required for the tracking hot path.
- **Automated Route Discovery**: Intercepts `routedetailslive` (and legacy `route-live-info`) endpoints via headless browser events for scheduler/route data.
- **Cross-Platform Browser Discovery**: Automatically locates Chrome, Chromium, Brave, or Microsoft Edge on Windows, macOS, and Linux without hardcoded paths.
- **Strict Concurrency Control**: Built-in worker semaphore and connection pooling to prevent CPU/memory exhaustion and browser-spawn denial of service.
- **Clean Architecture**: Separation of concerns across handlers, business logic, Chalo client, browser managers, OAuth, and middleware.
- **Defensive Error Handling**: Zero server crashes; standard JSON error envelopes with typed codes (`INVALID_BUS_CODE`, `CHALO_TIMEOUT`, `BUS_NOT_FOUND`, etc.).
- **Security Hardening**:
  - Strict bus code input validation against SSRF and command/path injection.
  - CSRF-resistant Google OAuth with HMAC-SHA256 state signing and secure HTTP-only cookies.
  - Rate limiting via token bucket algorithm.
  - Defensive security headers (`X-Content-Type-Options`, `X-Frame-Options`, `CSP`).
  - Request body size limits.
- **Structured Observability**: Context-aware logging using Go's standard `log/slog` with request IDs (`X-Request-ID`), latency tracking, and sensitive data redaction.
- **Backward Compatibility**: Full support for legacy endpoints (`/trackroute`, `/trackxhr`, `/testroute`, `/get_bus_url`, `/auth/google/*`) alongside modern `/api/v1` routes.
- **Comprehensive Test Coverage**: Complete unit test suite with HTTP mocks, testing validation, error paths, browser management, and middlewares.

---

## Architecture

```
                                  +-----------------------+
                                  |   HTTP Client / Web   |
                                  +-----------+-----------+
                                              |
                                      [Port 8080]
                                              |
                     +------------------------v------------------------+
                     |                 Middleware Pipeline              |
                     |  (RequestID, Logger, Recovery, Security, CORS,  |
                     |                   RateLimiter)                  |
                     +------------------------+------------------------+
                                              |
                     +------------------------v------------------------+
                     |                  HTTP Handlers                  |
                     |  HealthHandler    BusHandler       AuthHandler  |
                     +--------+---------------+----------------+-------+
                              |               |                |
                     +--------v------+ +------v-------+ +------v-------+
                     | Health Status | | ChaloService | | OAuthService |
                     +---------------+ +------+-------+ +--------------+
                                              |
                      +-----------------------+-----------------------+
                      |                                               |
             +--------v--------+                             +--------v--------+
             |   Chalo Client  |                             |  Chalo Scraper  |
             | (HTTP/REST API) |                             |   (chromedp)    |
             +--------+--------+                             +--------+--------+
                      |                                               |
              [Pooled Transports]                           [Browser Manager Pool]
                      |                                               |
                      v                                               v
             +-----------------+                             +-----------------+
             |   chalo.com     |                             | Headless Chrome |
             |   Backend API   |                             | / Brave / Edge  |
             +-----------------+                             +-----------------+
```

---

## Requirements

- **Go**: Version 1.26 or later (compatible with Go 1.22+).
- **Browser**: Any Chromium-based browser installed locally:
  - Google Chrome / Chrome Stable
  - Chromium
  - Brave Browser
  - Microsoft Edge
- **Operating System**: Windows 10/11, macOS (Intel/Apple Silicon), or Linux (Ubuntu, Debian, Fedora, CentOS, etc.).

---

## Installation

Clone the repository and download dependencies:

```bash
git clone https://github.com/R-zin/Ethiyo.git
cd Ethiyo
go mod download
```

Build the server binary:

```bash
# Using Makefile
make build

# Or directly using Go
go build -o bin/ethiyo ./cmd/server
```

---

## Configuration

Ethiyo is configured completely via environment variables. Create a `.env` file from the provided template:

```bash
cp .env.example .env
```

### Environment Variables

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | string | `8080` | Port the HTTP server listens on |
| `ENV` | string | `development` | Environment mode (`development` or `production`) |
| `CHALO_BASE_URL` | string | `https://chalo.com` | Base URL for Chalo services |
| `REQUEST_TIMEOUT` | duration | `15s` | Timeout for external HTTP requests |
| `BROWSER_TIMEOUT` | duration | `15s` | Timeout for browser-based discovery operations |
| `BROWSER_PATH` | string | *(auto)* | Path to browser binary (auto-discovered if empty) |
| `BROWSER_HEADLESS` | bool | `true` | Run browser in headless mode without GUI |
| `BROWSER_MAX_CONCURRENCY` | int | `4` | Maximum concurrent browser processes |
| `GOOGLE_CLIENT_ID` | string | `""` | Google OAuth2 Client ID |
| `GOOGLE_CLIENT_SECRET` | string | `""` | Google OAuth2 Client Secret |
| `GOOGLE_REDIRECT_URL` | string | `""` | OAuth callback redirect URL |
| `OAUTH_STATE_SECRET` | string | *(random)* | HMAC secret for CSRF state validation |
| `CORS_ALLOWED_ORIGINS` | string | `*` | Comma-separated list of allowed origins |
| `RATE_LIMIT_RPS` | float | `10.0` | In-memory token bucket refill rate (requests/sec) |
| `RATE_LIMIT_BURST` | int | `20` | In-memory token bucket maximum burst capacity |
| `LIVE_CACHE_TTL` | duration | `5s` | How long a live position is served from memory (`0s` disables storage; requests still coalesce) |
| `ROUTE_CACHE_TTL` | duration | `10m` | How long a browser-discovered route URL is reused |
| `STREAM_POLL_INTERVAL` | duration | `10s` | How often each streamed bus is polled upstream (shared by all its viewers) |
| `STREAM_MAX_CLIENTS` | int | `500` | Maximum concurrent live-stream connections |
| `STREAM_HEARTBEAT` | duration | `15s` | Keep-alive comment interval on idle streams |
| `STREAM_MAX_LIFETIME` | duration | `30m` | Maximum duration of one stream connection (clients reconnect automatically) |
| `METRICS_ENABLED` | bool | `true` | Expose Prometheus metrics at `/metrics` |
| `UPSTREAM_BREAKER_THRESHOLD` | int | `5` | Consecutive Chalo failures that open the circuit breaker |
| `UPSTREAM_BREAKER_COOLDOWN` | duration | `30s` | How long calls to Chalo are paused before a single trial request |
| `LAST_KNOWN_TTL` | duration | `15m` | How long a bus's last known position may be served (flagged `degraded`) while Chalo fails; `0s` disables |

Unparseable values are logged as a warning and the default is used.

---

## Running Locally

To start the backend server in development mode:

```bash
# Using make
make run

# Or directly with Go
go run ./cmd/server
```

### Running the Frontend

Ethiyo includes a dedicated modern Nordic Minimalist React 19 web application in `frontend/`:

```bash
# Run Vite development server on http://localhost:3000 (proxies /api to :8080)
make frontend-dev

# Run frontend test suite (Vitest + React Testing Library)
make frontend-test

# Build production bundle
make frontend-build

# Run automated headless Chromium E2E verification
make e2e-test
```

Example startup log:

```json
{"time":"2026-09-07T15:16:09.123+05:30","level":"INFO","msg":"initializing Ethiyo server..."}
{"time":"2026-09-07T15:16:09.124+05:30","level":"INFO","msg":"browser executable located","path":"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"}
{"time":"2026-09-07T15:16:09.125+05:30","level":"INFO","msg":"server listening","port":"8080","env":"development"}
```

---

## API Documentation

### 1. Health Checks

#### `GET /health` or `GET /api/v1/health`
Checks server status and browser availability. Also reports Chalo's health. It always returns 200 while the process can serve requests, so a Chalo outage never makes a load balancer restart a healthy instance:

```json
{
  "status": "healthy",
  "browser": true,
  "degraded": false,
  "upstream": { "state": "closed", "consecutive_failures": 0, "last_success": "2026-11-20T04:39:35Z" }
}
```

`upstream.state` is `closed` (healthy), `open` (calls paused; `retry_after_seconds` says for how long) or `half_open` (the next call is a trial).

### 2. V1 Bus Transit APIs

#### `GET /api/v1/bus/:buscode/track`
Returns the live position snapshot and tracking URL for the specified bus code, fetched directly from Chalo's vehicle-tracking API (no browser required). Includes real-time GPS coordinates, speed, route name, and next/previous stop when the bus is actively reporting.

When Chalo is failing, the bus's last known position is returned with `"degraded": true` (for up to `LAST_KNOWN_TTL`). A bus with no recent position gets `503 UPSTREAM_UNAVAILABLE`, with `Retry-After` set to the remaining breaker cooldown.

#### `GET /api/v1/bus/:buscode/route`
Returns both the live tracking URL and the live scheduler route URL.

#### `GET /api/v1/bus/:buscode/url`
Returns a compact tracking URL payload. An unknown bus returns `404 BUS_NOT_FOUND`.

#### `GET /api/v1/bus/:buscode/history`
Returns the recent breadcrumb trail of a bus (oldest point first, up to 60 fixes) and the distance travelled along it. A bus that has just gone offline still returns its last known trail.

Add `?format=geojson` (an RFC 7946 FeatureCollection with a LineString plus one Point per fix) or `?format=gpx` (a GPX 1.1 track) to download the trail as a file. Any other value returns `400 INVALID_FORMAT`.

#### `GET /api/v1/bus/:buscode/stream`
A [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) stream of live positions. Validation, unknown-bus and capacity errors are returned as normal JSON errors *before* the stream starts. After that the stream emits:

| Event | Data |
| :--- | :--- |
| `position` | Same object as the `data` of `/track`. Sent once on connect, then whenever the position changes. |
| `upstream_error` | `{"code": "...", "message": "..."}` when Chalo fails. The stream stays open and recovers automatically. |

A `: ping` comment is sent every `STREAM_HEARTBEAT`. When capacity is full the endpoint returns `503 STREAM_CAPACITY` with `Retry-After`, and the web app falls back to polling.

```bash
curl -N http://localhost:8080/api/v1/bus/KS602/stream
```

#### `GET /api/v1/buses/track?codes=KS602,DL1PC0001`
Live positions for up to 20 buses (duplicates are ignored). Results come back in request order, one per bus, each with either `tracking` (the same object as `/track`) or `error`. One failing bus never fails the whole request:

```json
{
  "success": true,
  "data": {
    "buses": [
      { "bus_code": "KS602", "tracking": { "bus_code": "KS602", "live": { "latitude": 11.2059, "...": "..." } } },
      { "bus_code": "ZZZ9", "error": { "code": "BUS_NOT_FOUND", "message": "Bus route not found on Chalo" } }
    ]
  }
}
```

An invalid code returns `400 INVALID_BUS_CODE` naming the entry, and more than 20 returns `400 TOO_MANY_BUS_CODES`.

#### `GET /api/v1/buses/stream?codes=KS602,DL1PC0001`
One server-sent events connection for several buses. It emits `position` events (each includes `bus_code`) and `bus_error` events (`{"bus_code", "code", "message"}`). Each bus counts toward `STREAM_MAX_CLIENTS`; a stream that doesn't fit is rejected entirely (`503 STREAM_CAPACITY`) rather than half-admitted.

#### `GET /api/v1/openapi.json`
The OpenAPI 3.1 description of every `/api/v1` endpoint. Tests keep it in sync: a route or model field that isn't documented (or vice versa) fails the build.

### 3. Observability

#### `GET /metrics`
Prometheus text exposition (disable with `METRICS_ENABLED=false`). Labels use route templates (`/api/v1/bus/:buscode/track`), never raw paths, so cardinality stays bounded. `/metrics` and the health endpoints are exempt from rate limiting.

### 4. Authentication APIs

#### `GET /api/v1/auth/google/login` (or `/auth/google/login`)
Initiates the Google OAuth2 login flow with an HMAC-signed CSRF state parameter.

#### `GET /api/v1/auth/google/callback` (or `/auth/google/callback`)
Validates the CSRF state and exchanges the authorization code for an authenticated session.

### 5. Legacy Endpoints (Backward Compatible)

- `GET /trackroute?buscode=12345`: Requests public route page; returns `{"message": "ok"}`.
- `GET /trackxhr?buscode=12345`: Returns `{"message": "ok", "route": "...", "cookie": "...", "error": null}`.
- `GET /testroute?buscode=12345`: Discovers route details and returns parsed JSON data.
- `GET /get_bus_url?buscode=12345`: Returns `{"result": "..."}`.

---

## Example Requests

### Health Check
```bash
curl -X GET http://localhost:8080/health
```

### Live Bus Tracking (v1)
```bash
curl -X GET http://localhost:8080/api/v1/bus/KS602/track
```

### Route Details Discovery (v1)
```bash
curl -X GET http://localhost:8080/api/v1/bus/DL1PC0001/route
```

### Legacy Route Query
```bash
curl -X GET "http://localhost:8080/trackxhr?buscode=DL1PC0001"
```

---

## Example Responses

### Success Response (`GET /api/v1/bus/KS602/track`)
Returns the live position snapshot when the bus is actively reporting GPS.
```json
{
  "success": true,
  "data": {
    "bus_code": "KS602",
    "tracking_url": "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602",
    "live": {
      "bus_code": "KS602",
      "vehicle_code": "KL15A3159",
      "operator": "KSRTC",
      "route_name": "Kannur-Punalur",
      "latitude": 11.205833,
      "longitude": 75.811525,
      "speed": 0,
      "recorded_at": "2026-09-19T22:23:55+05:30",
      "next_stop": "Cheruvannur Koyas",
      "previous_stop": "Meenchanda Bypass",
      "live_tracking": true,
      "tracking_url": "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602",
      "heading": 92.4,
      "age_seconds": 12.3,
      "stale": false
    }
  }
}
```

`heading` (degrees clockwise from north) is omitted until the bus has been seen moving. `stale` becomes `true` when the fix is more than 3 minutes old.

If the vehicle has no live data (offline or unknown), the endpoint returns `404 BUS_NOT_FOUND`.

### Route Details (`GET /api/v1/bus/KS602/route`)
Route discovery still uses the headless browser to intercept the scheduler endpoint. It finishes as soon as `routedetailslive` is observed. `tracking_url` is the deterministic vehicle-tracking URL. Results are cached for `ROUTE_CACHE_TTL`.
```json
{
  "success": true,
  "data": {
    "bus_code": "KS602",
    "tracking_url": "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602",
    "route_url": "https://chalo.com/app/api/scheduler_v4/v4/pathanamthitta/routedetailslive?route_id=fwbQqYZf&day=saturday"
  }
}
```

### Error Response (`GET /api/v1/bus/invalid$$code/track` - 400 Bad Request)
```json
{
  "success": false,
  "error": {
    "code": "INVALID_BUS_CODE",
    "message": "buscode must be alphanumeric (hyphens and underscores allowed) and between 1 and 64 characters"
  }
}
```

### Rate Limited Response (429 Too Many Requests)
Includes a `Retry-After` header with the number of seconds until the next request is allowed.
```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests. Please try again later."
  }
}
```

---

## Live Data Source

Ethiyo's `/track` flow reads live positions directly from Chalo's vehicle-tracking API rather than scraping a browser-emitted XHR.

- **Endpoint**: `GET https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=<bus_code>` (direct HTTP, no headless browser on the hot path).
- **Why**: Chalo's public route pages no longer emit the legacy `vasudha/track/route-live-info` XHR that earlier versions intercepted. The `chatbot/raw` endpoint is what those pages now call for live data, keyed by the same bus code clients already supply.
- **Payload**: terse JSON fields — `sessionData.data.currentInfo` carries `lt`/`ln` (lat/lon), `pSp` (speed), `tS` (epoch-millis timestamp); `nextStop`/`previousStop`/`routeDetails`/`gpsData` carry trip context. `internal/chalo/client.go:ParseLiveTracking` decodes this into `models.BusLiveInfo`.
- **HTTP 202 quirk**: Chalo answers with `202 Accepted` for **both** known vehicles (full payload) and unknown/offline ones (`{"error":"gps data is not present..."}`). Status alone can't distinguish them, so the client returns the body for any non-error status and lets `ParseLiveTracking` classify (no position → `ErrBusNotFound` → HTTP 404). This behavior is pinned by tests.
- **Freshness**: positions update at the operator's telemetry cadence (KSRTC ≈ 30s). `recorded_at` reflects Chalo's `tS` timestamp, not fetch time.

Route *schedules* (`/route`) still use the headless browser to intercept `scheduler_v4/.../routedetailslive`, which remains browser-emitted. See [Limitations](#limitations).

---

## Authentication

Google OAuth2 authentication is optionally configured via `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `GOOGLE_REDIRECT_URL`.

When enabled:
1. A client visits `/api/v1/auth/google/login`.
2. The server generates a cryptographic state token containing a random nonce, Unix timestamp, and HMAC-SHA256 signature using `OAUTH_STATE_SECRET`.
3. The state is written to a temporary, `HttpOnly`, `SameSite=Lax` cookie (`ethiyo_oauth_state`) and sent in the redirect URL.
4. Google returns to `/api/v1/auth/google/callback?code=...&state=...`.
5. The server validates that the state signature is valid, has not expired (< 10 min), and matches the cookie.
6. The authorization code is exchanged for an access token via Google's OAuth endpoints.

---

## Browser Requirements

Ethiyo uses `github.com/chromedp/chromedp` to drive Chromium instances.

### Auto-Discovery Order
If `BROWSER_PATH` is not explicitly set, Ethiyo checks:
1. System `$PATH` for binaries: `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`, `brave-browser`, `brave`, `chrome`, `msedge`.
2. Standard platform installation folders:
   - **Windows**: `Program Files`, `Program Files (x86)`, and `%LOCALAPPDATA%` under Google, BraveSoftware, and Microsoft Edge.
   - **macOS**: `/Applications/Google Chrome.app`, `/Applications/Brave Browser.app`, `/Applications/Microsoft Edge.app`.
   - **Linux**: `/usr/bin/google-chrome`, `/usr/bin/chromium-browser`, `/snap/bin/chromium`, etc.

---

## Troubleshooting

### Browser Not Found Error
- **Symptom**: Warning `browser executable not found at startup` or errors during scraping.
- **Solution**: Install Google Chrome, Chromium, or Brave, or set `BROWSER_PATH`:
  ```bash
  export BROWSER_PATH="/usr/bin/google-chrome"
  ```

### Timed out waiting for Chalo response
- **Symptom**: HTTP 504 Gateway Timeout with code `CHALO_TIMEOUT`.
- **Solution**: Ensure internet access is available to `chalo.com`. Increase `BROWSER_TIMEOUT` in your `.env` (e.g., `BROWSER_TIMEOUT=25s`).

### Chalo Is Down / `degraded: true` Responses
- **Symptom**: Responses carry `"degraded": true`, the app shows "Chalo isn't responding", or `/health` reports `upstream.state: "open"`.
- **Meaning**: Chalo has failed `UPSTREAM_BREAKER_THRESHOLD` times in a row, so Ethiyo is serving last known positions and probing for recovery every `UPSTREAM_BREAKER_COOLDOWN`. It recovers on its own once Chalo answers again; `ethiyo_upstream_circuit_open` in `/metrics` tracks it.

### Browser Concurrency Limit Exceeded
- **Symptom**: HTTP 503 with code `BROWSER_BUSY`.
- **Solution**: Increase `BROWSER_MAX_CONCURRENCY` in `.env` if your host machine has sufficient RAM/CPU.

---

## Development

### Makefile Targets
```bash
make build       # Compile server binary to bin/ethiyo
make run         # Run server locally
make test        # Run all automated tests
make test-race   # Run all automated tests with race detector
make vet         # Run go vet analysis
make fmt         # Format all source files with gofmt
make fmt-check   # Check code formatting in CI
make clean       # Remove build and test artifacts
```

### Continuous Integration

GitHub Actions runs on pushes to `main` and `new`, and on pull requests targeting
`main`. The CI workflow verifies module consistency, formatting, vet results,
race-enabled tests, coverage, Staticcheck, and known Go vulnerabilities. It
also builds Linux amd64/arm64 and Windows amd64 binaries.

Coverage reports and platform-specific binaries are uploaded as workflow
artifacts and retained for 14 days. Pull requests additionally run GitHub's
dependency review against the proposed changes.

---

## Testing

Run the full automated test suite:

```bash
go test -v ./...
```

Run static analysis:

```bash
go vet ./...
```

Unit tests use `httptest` and in-memory mocks to isolate tests from live network dependencies.

### End-to-End

The E2E suite runs the real backend against `frontend/mock-chalo.cjs`. The mock reproduces Chalo's quirks: HTTP 202 for known *and* unknown buses, a bus that drives a loop, a route page that fires the scheduler XHR, and an outage switch (`/__mock/fail?on=1`). The suite drives headless Chromium through:

- live data, the SSE stream, the heading arrow and trail
- proximity and the arrival alert (including a real browser notification)
- My buses over one multi-bus stream
- a simulated Chalo outage (degraded banner, stopped-reporting alert, header status) and its recovery
- route discovery, error states and responsive layouts

With `PWA_URL` set, it also checks the installable production build: the manifest, the service worker, that no live data is cached, and the offline shell.

```bash
node frontend/mock-chalo.cjs &
CHALO_BASE_URL=http://localhost:9099 STREAM_POLL_INTERVAL=2s LIVE_CACHE_TTL=1s UPSTREAM_BREAKER_COOLDOWN=3s go run ./cmd/server &
make frontend-dev &
npm --prefix frontend run build && npm --prefix frontend run preview -- --port 4173 --strictPort &
PWA_URL=http://localhost:4173 make e2e-test
```

---

## Project Structure

```
Ethiyo/
├── .github/
│   └── workflows/
│       └── actions.yml      # GitHub Actions CI workflow (Quality, Security, E2E Chromium)
├── cmd/
│   └── server/
│       └── main.go          # Application entrypoint
├── frontend/                # Nordic Minimalist React 19 + TypeScript + Vite web application
│   ├── src/                 # Components (Header, Map, Tracker, SearchBar, States)
│   ├── public/              # Icons, PWA manifest and service worker (sw.js)
│   ├── browser-verification-suite.cjs # Headless Chromium DevTools CDP test runner
│   ├── mock-chalo.cjs       # Deterministic upstream test mock server for CI
│   ├── package.json         # Frontend dependencies and test scripts
│   └── vite.config.ts       # Vite configuration with proxy to Go backend
├── internal/
│   ├── apidocs/             # Embedded OpenAPI 3.1 document and its drift tests
│   ├── auth/                # Google OAuth2 and HMAC CSRF state validation
│   ├── breaker/             # Circuit breaker for calls to Chalo
│   ├── browser/             # Cross-platform browser locator and concurrency manager
│   ├── cache/               # Generic TTL cache with request coalescing
│   ├── chalo/               # Chalo client, scraper, regex patterns, and service
│   ├── config/              # Environment loading and configuration validation
│   ├── handlers/            # HTTP handlers for health, bus routes, SSE streams, and OAuth
│   ├── metrics/             # Dependency-free Prometheus exposition
│   ├── middleware/          # Logger, request ID, recovery, CORS, and rate limiting
│   ├── models/              # Domain models, bus validation, and response envelopes
│   ├── server/              # Server assembly and graceful shutdown lifecycle
│   ├── stream/              # Live-stream hub: shared per-bus pollers and fan-out
│   └── telemetry/           # Position history, heading and geo math
├── .env.example             # Example environment configuration
├── .gitignore               # Ignored files (binaries, secrets, IDE, OS)
├── .mcp.json                # Model Context Protocol configuration
├── go.mod                   # Go module definitions
├── go.sum                   # Checksums for direct and indirect dependencies
├── Makefile                 # Development, build, frontend, and E2E automation tasks
└── README.md                # Project documentation
```

---

## Security

1. **SSRF Prevention**: All route URLs fetched by the backend are strictly verified to ensure they belong to `chalo.com/app/api/`.
2. **Bus Code Sanitization**: Bus codes are restricted to alphanumeric characters, hyphens, and underscores between 1 and 64 characters.
3. **No Process Termination in Handlers**: Completely eliminated `log.Fatal` from request handling paths.
4. **OAuth CSRF Protection**: HMAC-SHA256 signed state tokens with expiration timestamps and `SameSite=Lax` cookies.
5. **No Secret Leaks**: Secrets and tokens are kept out of logs and client error responses.
6. **Rate Limiting**: Defends expensive headless browser operations against denial of service.

---

## Limitations

- **Chalo API Contract**: Live tracking depends on Chalo's `dashboard/chatbot/raw?vehicleNo=` endpoint. Chalo answers it with HTTP 202 for **both** known and unknown vehicles (distinguished only by the response body), which `internal/chalo/client.go` handles explicitly. If Chalo changes this endpoint or its payload shape (`lt`/`ln`/`tS`/`pSp` fields), update `ParseLiveTracking`.
- **GPS Reporting Cadence**: Positions update at the operator's telemetry cadence (KSRTC ≈ every 30s). A bus that stops reporting returns `404 BUS_NOT_FOUND`.
- **Route Discovery Requires Browser**: The `/route` flow still needs the headless browser to intercept `scheduler_v4/.../routedetailslive`. If Chalo stops issuing that request, update the regex patterns in `internal/chalo/regex.go`.
- **Per-Instance State**: The cache, position history and stream hub live in memory, so each replica has its own. Trails start empty after a restart and are not shared between instances.
- **ETA Is Approximate**: The web app's ETA is straight-line distance divided by current speed. It ignores the road network, stops and traffic, and is labelled as approximate.
- **No Per-Stop ETAs Yet**: Stop-level arrival times would need the stop list from Chalo's `routedetailslive` response. Its schema is undocumented and could not be inspected from the development environment, so it is not parsed yet rather than guessed at.
- **Alerts Need an Open Page**: Arrival and disruption alerts are evaluated in the browser, so they only fire while the app is open (in a tab or as an installed app). Background push would need a push server.
- **Headless Browser Overhead**: Headless browser automation requires approximately 50MB-100MB of RAM per active session. Ensure server sizing accounts for `BROWSER_MAX_CONCURRENCY`.

---

## Roadmap

- [ ] Add Redis-backed caching and a shared stream hub so multiple replicas share state.
- [x] Add Prometheus metrics exporter (`/metrics`) for latency histograms and browser pool utilization.
- [x] Stream real-time bus location updates directly to clients (implemented with server-sent events).
- [x] In-memory caching with request coalescing for live tracking and route discovery.
- [ ] Add Dockerfile and multi-stage container build with bundled Chromium.
- [ ] Per-stop ETAs from the `routedetailslive` stop list (needs real payload samples).
- [ ] Web Push so alerts arrive when the app is closed.
- [x] Saved buses with a multi-bus live stream, arrival and disruption alerts.
- [x] Circuit breaker with last-known-position fallback when Chalo fails.
- [x] OpenAPI 3.1 document, trail export (GeoJSON/GPX) and an installable PWA.
