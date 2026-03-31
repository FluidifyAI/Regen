/**
 * webhook-sustained.js — OPE-53
 *
 * Simulates a realistic sustained webhook load: 50 VUs sending Prometheus
 * Alertmanager payloads for 5 minutes. Validates that p99 latency stays
 * under 200 ms throughout.
 *
 * Run:
 *   k6 run load-tests/webhook-sustained.js
 *   BASE_URL=http://prod-host:8080 k6 run load-tests/webhook-sustained.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('webhook_errors');
const webhookDuration = new Trend('webhook_duration_ms', true);

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // ramp up
    { duration: '4m',  target: 50 },   // sustained load
    { duration: '30s', target: 0  },   // ramp down
  ],
  thresholds: {
    // Core SLA: webhook p99 must be under 200 ms
    'http_req_duration': ['p(99)<200'],
    'webhook_duration_ms': ['p(99)<200'],
    // No server errors allowed
    'http_req_failed{status:500}': ['rate<0.001'],
    'http_req_failed{status:503}': ['rate<0.001'],
    // Our custom metric — only fail if actual errors (not just checks)
    webhook_errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Realistic Alertmanager payload — mimics what Prometheus sends on a firing alert
function makePrometheusPayload(vu, iter) {
  const alertName = `LoadTestAlert_VU${vu}_${iter % 10}`;
  return JSON.stringify({
    receiver: 'fluidify-regen',
    status: 'firing',
    alerts: [
      {
        status: 'firing',
        labels: {
          alertname: alertName,
          severity: iter % 5 === 0 ? 'critical' : 'warning',
          instance: `web-0${(vu % 3) + 1}`,
          job: 'load-test',
        },
        annotations: {
          summary: `Load test alert from VU ${vu}`,
          description: `Iteration ${iter}`,
        },
        startsAt: new Date().toISOString(),
        fingerprint: `lt_${vu}_${iter % 10}`,
      },
    ],
    groupLabels: { alertname: alertName },
    commonLabels: { severity: 'warning', job: 'load-test' },
    commonAnnotations: {},
    externalURL: 'http://alertmanager:9093',
    version: '4',
    groupKey: `{}/{}:{alertname="${alertName}"}`,
  });
}

export default function () {
  const payload = makePrometheusPayload(__VU, __ITER);

  // Use unique per-VU source IP so each VU gets its own rate-limit bucket,
  // matching the production topology where each alert source has its own IP.
  const res = http.post(
    `${BASE_URL}/api/v1/webhooks/prometheus`,
    payload,
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Forwarded-For': `10.0.${Math.floor(__VU / 256)}.${__VU % 256}`,
      },
      timeout: '5s',
    }
  );

  webhookDuration.add(res.timings.duration);

  const ok = check(res, {
    'status 200': (r) => r.status === 200,
    'received > 0': (r) => {
      try {
        return JSON.parse(r.body).received > 0;
      } catch (_) {
        return false;
      }
    },
  });

  if (!ok) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }

  // Realistic inter-request think time: 100–500 ms
  sleep(0.1 + Math.random() * 0.4);
}
