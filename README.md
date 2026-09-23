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

---

## Running Locally

To start the server in development mode:

```bash
# Using make
make run

# Or directly with Go
go run ./cmd/server
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
Checks server status and browser availability.

### 2. V1 Bus Transit APIs

#### `GET /api/v1/bus/:buscode/track`
Returns the live position snapshot and tracking URL for the specified bus code, fetched directly from Chalo's vehicle-tracking API (no browser required). Includes real-time GPS coordinates, speed, route name, and next/previous stop when the bus is actively reporting.

#### `GET /api/v1/bus/:buscode/route`
Returns both the live tracking URL and the live scheduler route URL.

#### `GET /api/v1/bus/:buscode/url`
Returns a compact tracking URL payload.

### 3. Authentication APIs

#### `GET /api/v1/auth/google/login` (or `/auth/google/login`)
Initiates the Google OAuth2 login flow with an HMAC-signed CSRF state parameter.

#### `GET /api/v1/auth/google/callback` (or `/auth/google/callback`)
Validates the CSRF state and exchanges the authorization code for an authenticated session.

### 4. Legacy Endpoints (Backward Compatible)

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
      "tracking_url": "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602"
    }
  }
}
```

If the vehicle has no live data (offline or unknown), the endpoint returns `404 BUS_NOT_FOUND`.

### Route Details (`GET /api/v1/bus/KS602/route`)
Route discovery still uses the headless browser to intercept the scheduler endpoint.
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

---

## Project Structure

```
Ethiyo/
├── .github/
│   └── workflows/
│       └── actions.yml      # GitHub Actions CI workflow
├── cmd/
│   └── server/
│       └── main.go          # Application entrypoint
├── internal/
│   ├── auth/                # Google OAuth2 and HMAC CSRF state validation
│   ├── browser/             # Cross-platform browser locator and concurrency manager
│   ├── chalo/               # Chalo client, scraper, regex patterns, and service
│   ├── config/              # Environment loading and configuration validation
│   ├── handlers/            # HTTP handlers for health, bus routes, and OAuth
│   ├── middleware/          # Logger, request ID, recovery, CORS, and rate limiting
│   ├── models/              # Domain models, bus validation, and response envelopes
│   └── server/              # Server assembly and graceful shutdown lifecycle
├── .env.example             # Example environment configuration
├── .gitignore               # Ignored files (binaries, secrets, IDE, OS)
├── go.mod                   # Go module definitions
├── go.sum                   # Checksums for direct and indirect dependencies
├── Makefile                 # Development and build tasks
├── main.go                  # Root runner shim
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
- **Headless Browser Overhead**: Headless browser automation requires approximately 50MB-100MB of RAM per active session. Ensure server sizing accounts for `BROWSER_MAX_CONCURRENCY`.

---

## Roadmap

- [ ] Add Redis-backed caching for route URLs and live tracking endpoints with TTL.
- [ ] Add Prometheus metrics exporter (`/metrics`) for latency histograms and browser pool utilization.
- [ ] Support WebSocket streaming of real-time bus location updates directly to clients.
- [ ] Add Dockerfile and multi-stage container build with bundled Chromium.
