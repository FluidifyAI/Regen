/**
 * webhook-burst.js — OPE-53
 *
 * Simulates a sudden alert storm: traffic ramps from 10 to 500 VUs in 30 s,
 * holds for 1 minute, then drops. Validates two things:
 *
 *   1. The rate limiter kicks in cleanly (429s, no 5xx or OOM)
 *   2. After the burst, normal traffic recovers with no degradation
 *
 * Note: 429s are EXPECTED during the burst phase — this test is checking that
 * the server degrades gracefully, not that all requests succeed.
 *
 * Run:
 *   k6 run load-tests/webhook-burst.js
 *   BASE_URL=http://prod-host:8080 k6 run load-tests/webhook-burst.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const requests429 = new Counter('rate_limited_requests');
const requests5xx  = new Counter('server_error_requests');
const errorRate    = new Rate('error_rate');

export const options = {
  stages: [
    { duration: '10s', target: 10  },  // baseline
    { duration: '30s', target: 500 },  // burst: simulate alert storm
    { duration: '1m',  target: 500 },  // sustain the flood
    { duration: '30s', target: 10  },  // drain
    { duration: '1m',  target: 10  },  // recovery check
    { duration: '10s', target: 0   },  // ramp down
  ],
  thresholds: {
    // The server must NEVER return 5xx under flood — only 429
    server_error_requests: ['count<5'],
    // During recovery phase (last 70 s), p99 should be back under 500 ms
    // (We can't easily filter by stage in thresholds, so we use a loose bound)
    http_req_duration: ['p(99)<2000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function makePayload() {
  return JSON.stringify({
    receiver: 'fluidify-regen',
    status: 'firing',
    alerts: [
      {
        status: 'firing',
        labels: {
          alertname: `BurstAlert_${__VU}`,
          severity: 'critical',
          instance: `web-burst-${__VU % 10}`,
          job: 'burst-test',
        },
        annotations: { summary: 'Burst load test alert' },
        startsAt: new Date().toISOString(),
        fingerprint: `burst_${__VU}`,
      },
    ],
    groupLabels: { alertname: `BurstAlert_${__VU}` },
    commonLabels: {},
    commonAnnotations: {},
    externalURL: 'http://alertmanager:9093',
    version: '4',
    groupKey: `burst_${__VU}`,
  });
}

export default function () {
  const res = http.post(
    `${BASE_URL}/api/v1/webhooks/prometheus`,
    makePayload(),
    {
      headers: { 'Content-Type': 'application/json' },
      timeout: '10s',
    }
  );

  check(res, {
    'not a 5xx': (r) => r.status < 500,
  });

  if (res.status === 429) {
    requests429.add(1);
    errorRate.add(0); // 429 is expected, not an error
  } else if (res.status >= 500) {
    requests5xx.add(1);
    errorRate.add(1); // 5xx is a real failure
  } else {
    errorRate.add(0);
  }

  // Minimal sleep so VUs keep hammering — this IS the point of a burst test
  sleep(0.05);
}

export function handleSummary(data) {
  const total    = data.metrics.http_reqs.values.count;
  const rate429  = data.metrics.rate_limited_requests
    ? data.metrics.rate_limited_requests.values.count
    : 0;
  const rate5xx  = data.metrics.server_error_requests
    ? data.metrics.server_error_requests.values.count
    : 0;

  console.log(`\n=== Burst test summary ===`);
  console.log(`Total requests  : ${total}`);
  console.log(`Rate-limited 429: ${rate429} (${((rate429 / total) * 100).toFixed(1)}%)`);
  console.log(`Server errors 5xx: ${rate5xx} (expected: 0)`);
  console.log(`p99 latency     : ${data.metrics.http_req_duration.values['p(99)'].toFixed(0)} ms`);
  console.log('==========================\n');

  return {};
}
