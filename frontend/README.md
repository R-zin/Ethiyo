# Ethiyo Transit Frontend (Nordic Minimalist UI)

A dedicated, production-grade transit telemetry dashboard for the Ethiyo backend, engineered with a minimalistic Nordic / Scandinavian design aesthetic.

---

## Design System & Theme

- **Palette**: Inspired by the Nordic natural landscape:
  - *Snow Storm* (`nordic-50` to `nordic-200`): Soft, natural off-whites and cool slates.
  - *Polar Night* (`nordic-900` to `nordic-950`): Deep, calm nocturnal slates.
  - *Glacier Frost* (`frost-500` to `frost-700`): Muted Arctic blues for focal telemetry and navigation pins.
  - *Spruce Pine* (`pine-500` to `pine-600`): Muted evergreen for active reporting and health indicators.
  - *Lingonberry* (`berry-500` to `berry-600`): Subdued crimson for error badges and alert states.
- **Typography**: Clean geometric typography using *Inter* and tabular numerals (*JetBrains Mono*) for speed, GPS coordinates, and latency indicators.
- **Restraint**: Zero arbitrary purple gradients, zero shadow bloat, and no generic template clutter. Every element serves an informational purpose.

---

## Features

- **Live GPS Telemetry**: Real-time vehicle speed gauge, coordinates, route name, next stop, and previous stop.
- **Interactive Nordic Map**: Smooth Leaflet map integration with custom animated bus pins, dark-mode tile inversion, coordinate HUD, and "recenter on vehicle" controls.
- **Smart Route Search**: Instant search with keyboard shortcut (`/`), quick presets (`KS602`, `DL1PC0001`, `route_500`), and recent history persistence.
- **Autonomous Polling**: Configurable 15s auto-refresh with smooth countdown ring and instant manual refresh.
- **Route Scheduler Inspector**: Deep-discovery view inspecting raw scheduler URLs (`routedetailslive`) and tracking endpoints (`route-live-info`) with one-click copy.
- **Real-Time API Health**: Live latency and status monitoring connected to the Go backend (`/health`).
- **Complete State Coverage**: Fully designed empty states, calm pulsing skeleton loaders, and actionable error alerts.
- **Accessibility**: WCAG 2.1 AA compliant, screen-reader friendly with semantic HTML and `aria-live` announcements.

---

## Tech Stack

- **Framework**: React 19 + TypeScript
- **Bundler**: Vite 6
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
- **Mapping**: Leaflet
- **Testing**: Vitest + React Testing Library

---

## Getting Started

### Development
```bash
# Start frontend dev server on http://localhost:3000
npm run dev
```

The dev server automatically proxies API requests (`/api` and `/health`) to `http://localhost:8080`.

### Testing
```bash
# Run unit tests
npm test
```

### Production Build
```bash
# Build production bundle into dist/
npm run build
```

---

## Environment Variables

| Variable | Description | Default |
| :--- | :--- | :--- |
| `VITE_API_URL` | Base URL of the Ethiyo Go backend | `""` (uses current origin or Vite proxy) |
