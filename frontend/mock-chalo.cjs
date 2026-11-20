// Deterministic stand-in for chalo.com used by the E2E suite and CI.
//
// It reproduces the upstream behaviours Ethiyo depends on, including the
// awkward ones, so E2E exercises the real code paths:
//   * /app/api/dashboard/chatbot/raw answers HTTP 202 for BOTH known vehicles
//     (full payload) and unknown ones (an error body) — just like Chalo.
//   * Known buses drive a closed rectangular loop (north, east, south, west),
//     so heading, breadcrumb trail and distance are all observable, and the
//     path never "teleports" back to its start.
//   * /app/public-route/:code serves a page that fires the scheduler
//     routedetailslive XHR, so headless-browser route discovery works.
//   * GET /__mock/fail?on=1 makes the live endpoint answer 503 (an outage)
//     until /__mock/fail?on=0, to exercise degradation and recovery.
const http = require('http');

const PORT = Number(process.env.MOCK_CHALO_PORT || 9099);
// A new GPS fix is "recorded" every STEP_MS, like an operator's telemetry cadence.
const STEP_MS = Number(process.env.MOCK_STEP_MS || 3000);

const KNOWN = {
  KS602: { vehicleCode: 'KL15A3159', operator: 'KSRTC', route: 'Kannur-Punalur', lat: 11.2059, lon: 75.8115 },
  DL1PC0001: { vehicleCode: 'DL1PC0001', operator: 'DTC', route: 'Ring Road Express', lat: 28.6139, lon: 77.209 },
  ROUTE_500: { vehicleCode: 'KA01F0500', operator: 'BMTC', route: 'Outer Ring Road 500', lat: 12.9716, lon: 77.5946 },
};

function positionFor(bus, now) {
  const step = Math.floor(now / STEP_MS);
  const phase = step % 40; // four legs of 10 steps (~100 m each)
  const t = phase % 10;
  const [north, east] = [
    [t, 0], // heading north
    [10, t], // east
    [10 - t, 10], // south
    [0, 10 - t], // west, back to the start
  ][Math.floor(phase / 10)];
  return { lat: bus.lat + north * 0.0009, lon: bus.lon + east * 0.0009, ts: step * STEP_MS };
}

function sendJSON(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

let failing = false;

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);

  if (url.pathname === '/__mock/fail') {
    failing = url.searchParams.get('on') === '1';
    sendJSON(res, 200, { failing });
    return;
  }

  if (url.pathname === '/app/api/dashboard/chatbot/raw') {
    if (failing) {
      res.writeHead(503, { 'Content-Type': 'text/plain' });
      res.end('Service Unavailable');
      return;
    }
    const vehicleNo = (url.searchParams.get('vehicleNo') || '').toUpperCase();
    const bus = KNOWN[vehicleNo];
    if (!bus) {
      sendJSON(res, 202, { error: `gps data is not present for ${vehicleNo} vehicle` });
      return;
    }
    const pos = positionFor(bus, Date.now());
    sendJSON(res, 202, {
      gpsData: {
        vehicleCode: bus.vehicleCode,
        operator: { name: bus.operator },
        city: { liveTracking: true },
      },
      sessionData: {
        data: {
          currentInfo: { lt: pos.lat, ln: pos.lon, pSp: 28.5, tS: pos.ts },
          nextStop: { name: 'Central Silk Board' },
          previousStop: { name: 'HSR Layout' },
          routeDetails: { routeName: bus.route },
        },
      },
    });
    return;
  }

  if (url.pathname.startsWith('/app/public-route/')) {
    const code = encodeURIComponent(url.pathname.split('/').pop() || '');
    const routeURL = `/app/api/scheduler_v4/v4/testcity/routedetailslive?route_id=${code}&day=monday`;
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(`<!doctype html><html><body>Route ${code}
      <script>fetch(${JSON.stringify(routeURL)}).catch(function () {});</script>
    </body></html>`);
    return;
  }

  if (url.pathname.includes('/routedetailslive')) {
    sendJSON(res, 200, {
      routeId: url.searchParams.get('route_id'),
      title: 'Outer Ring Road Expressway',
      stops: [
        { stopId: 's1', name: 'Hebbal Flyover' },
        { stopId: 's2', name: 'Silk Board Junction' },
      ],
    });
    return;
  }

  sendJSON(res, 404, { error: 'not found' });
});

server.listen(PORT, () => {
  console.log(`Mock Chalo upstream server listening on port ${PORT}`);
});

process.on('SIGTERM', () => {
  server.close(() => process.exit(0));
});
