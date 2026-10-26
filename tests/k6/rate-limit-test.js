import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const successful200 = new Counter('status_200_count');
const rateLimited429 = new Counter('status_429_count');

export const options = {
  scenarios: {
    burst_flood: {
      executor: 'constant-arrival-rate',
      rate: 50,          // 50 requests per second (exceeds default burst=20, rate=10)
      timeUnit: '1s',
      duration: '5s',
      preAllocatedVUs: 10,
      maxVUs: 30,
    },
  },
  thresholds: {
    'status_429_count': ['count>10'], // Confirms rate limiter actively protected the backend
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/health`);

  if (res.status === 200) {
    successful200.add(1);
    check(res, {
      'status is 200': r => r.status === 200,
    });
  } else if (res.status === 429) {
    rateLimited429.add(1);
    check(res, {
      'status is 429': r => r.status === 429,
      'code is RATE_LIMIT_EXCEEDED': r => {
        try {
          const body = JSON.parse(r.body);
          return body.error && body.error.code === 'RATE_LIMIT_EXCEEDED';
        } catch {
          return false;
        }
      },
    });
  }
}
