import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
const healthDuration = new Trend('health_duration');
const busUrlDuration = new Trend('bus_url_duration');
const busTrackDuration = new Trend('bus_track_duration');
const validationDuration = new Trend('validation_duration');
const successfulRequests = new Counter('successful_requests');
const rateLimitedRequests = new Counter('rate_limited_requests');
const errorRate = new Rate('custom_error_rate');

export const options = {
  vus: 3,
  duration: '15s',
  thresholds: {
    'http_req_duration{endpoint:health}': ['p(95)<50'],
    'http_req_duration{endpoint:bus_url}': ['p(95)<600'], // External transit API round-trip SLA
    'custom_error_rate': ['rate<0.05'], // Less than 5% non-expected errors
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

export default function () {
  // 1. Health check endpoint
  group('Health Check', function () {
    const res = http.get(`${BASE_URL}/health`, {
      tags: { endpoint: 'health' },
    });
    healthDuration.add(res.timings.duration);

    const isOk = check(res, {
      'status is 200': r => r.status === 200,
      'status is healthy': r => {
        try {
          const body = JSON.parse(r.body);
          return body.status === 'healthy';
        } catch {
          return false;
        }
      },
    });

    if (isOk) {
      successfulRequests.add(1);
      errorRate.add(0);
    } else {
      errorRate.add(1);
    }
  });

  sleep(0.2);

  // 2. Bus URL metadata generation (hot path)
  group('Bus URL Discovery', function () {
    const res = http.get(`${BASE_URL}/api/v1/bus/KS602/url`, {
      tags: { endpoint: 'bus_url' },
    });
    busUrlDuration.add(res.timings.duration);

    const isOk = check(res, {
      'status is 200': r => r.status === 200,
      'has valid tracking url': r => {
        try {
          const body = JSON.parse(r.body);
          return body.success && body.data.tracking_url.includes('vehicleNo=KS602');
        } catch {
          return false;
        }
      },
    });

    if (isOk) {
      successfulRequests.add(1);
      errorRate.add(0);
    } else {
      errorRate.add(1);
    }
  });

  sleep(0.2);

  // 3. Input validation defense
  group('Input Validation Defense', function () {
    const res = http.get(`${BASE_URL}/api/v1/bus/bad$$injection/url`, {
      tags: { endpoint: 'validation' },
    });
    validationDuration.add(res.timings.duration);

    const isRejected = check(res, {
      'status is 400': r => r.status === 400,
      'returns INVALID_BUS_CODE': r => {
        try {
          const body = JSON.parse(r.body);
          return body.error && body.error.code === 'INVALID_BUS_CODE';
        } catch {
          return false;
        }
      },
    });

    if (isRejected) {
      successfulRequests.add(1);
      errorRate.add(0);
    } else {
      errorRate.add(1);
    }
  });

  sleep(0.2);

  // 4. Live Tracking Endpoint (accepts 200 or 404 for unknown bus)
  group('Bus Live Tracking Query', function () {
    const res = http.get(`${BASE_URL}/api/v1/bus/KS602/track`, {
      tags: { endpoint: 'bus_track' },
    });
    busTrackDuration.add(res.timings.duration);

    if (res.status === 429) {
      rateLimitedRequests.add(1);
    }

    const isValidResponse = check(res, {
      'status is 200 or 404': r => r.status === 200 || r.status === 404,
    });

    if (isValidResponse) {
      successfulRequests.add(1);
      errorRate.add(0);
    } else {
      errorRate.add(1);
    }
  });

  sleep(0.5);
}
