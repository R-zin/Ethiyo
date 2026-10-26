import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Metrics
const healthTrend = new Trend('ttfb_health');
const busUrlTrend = new Trend('ttfb_bus_url');
const validationTrend = new Trend('ttfb_validation');
const rateLimitedCounter = new Counter('rate_limited_count');
const errorRate = new Rate('unexpected_error_rate');

export const options = {
  stages: [
    { duration: '10s', target: 5 },   // Ramp-up to 5 users
    { duration: '20s', target: 15 },  // Moderate traffic
    { duration: '15s', target: 25 },  // Peak traffic
    { duration: '10s', target: 0 },   // Cool-down
  ],
  thresholds: {
    'ttfb_health': ['p(95)<30'],
    'ttfb_bus_url': ['p(95)<600'], // Outbound upstream Chalo round-trip SLA
    'unexpected_error_rate': ['rate<0.02'], // Max 2% unexpected errors
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

const SAMPLE_BUS_CODES = ['KS602', 'DL1PC0001', 'route_500', 'KA01F1234', 'MH02AB5678'];

export default function () {
  const busCode = SAMPLE_BUS_CODES[Math.floor(Math.random() * SAMPLE_BUS_CODES.length)];

  // 1. Health check probe
  group('Health Check', function () {
    const res = http.get(`${BASE_URL}/health`);
    healthTrend.add(res.timings.duration);

    if (res.status === 429) {
      rateLimitedCounter.add(1);
    } else {
      const ok = check(res, {
        'health status 200': r => r.status === 200,
      });
      errorRate.add(!ok);
    }
  });

  sleep(0.1);

  // 2. Bus URL generation (high throughput hot path)
  group('Bus URL Generation', function () {
    const res = http.get(`${BASE_URL}/api/v1/bus/${busCode}/url`);
    busUrlTrend.add(res.timings.duration);

    if (res.status === 429) {
      rateLimitedCounter.add(1);
    } else {
      const ok = check(res, {
        'url status 200': r => r.status === 200,
        'payload contains bus_code': r => {
          try {
            const body = JSON.parse(r.body);
            return body.data && body.data.bus_code === busCode;
          } catch {
            return false;
          }
        },
      });
      errorRate.add(!ok);
    }
  });

  sleep(0.1);

  // 3. Security input validation check
  group('Input Sanitization', function () {
    const res = http.get(`${BASE_URL}/api/v1/bus/bad$$injection/url`);
    validationTrend.add(res.timings.duration);

    if (res.status === 429) {
      rateLimitedCounter.add(1);
    } else {
      const ok = check(res, {
        'invalid input returns 400': r => r.status === 400,
        'error code is INVALID_BUS_CODE': r => {
          try {
            const body = JSON.parse(r.body);
            return body.error && body.error.code === 'INVALID_BUS_CODE';
          } catch {
            return false;
          }
        },
      });
      errorRate.add(!ok);
    }
  });

  sleep(0.2);
}
