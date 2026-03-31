/**
 * api-read.js — OPE-53
 *
 * Simulates concurrent read traffic against the incident management API:
 *   - GET /api/v1/incidents          (list view — heaviest query)
 *   - GET /api/v1/incidents/:id      (detail view — uses UUID from list)
 *   - GET /api/v1/alerts             (alert list)
 *   - GET /health                    (liveness probe — should always be <5 ms)
 *
 * Requires a valid session token. Set via env var:
 *   AUTH_TOKEN=<token> k6 run load-tests/api-read.js
 *
 * To get a token:
 *   curl -s -X POST http://localhost:8080/api/v1/auth/login \
 *     -H 'Content-Type: application/json' \
 *     -d '{"email":"admin@example.com","password":"your-password"}' \
 *     | jq -r '.token'
 *
 * Run:
 *   AUTH_TOKEN=xxx k6 run load-tests/api-read.js
 *   BASE_URL=http://prod-host:8080 AUTH_TOKEN=xxx k6 run load-tests/api-read.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const listLatency   = new Trend('incident_list_ms', true);
const detailLatency = new Trend('incident_detail_ms', true);
const alertLatency  = new Trend('alert_list_ms', true);
const errorRate     = new Rate('api_errors');

export const options = {
  stages: [
    { duration: '30s', target: 20 },  // ramp up
    { duration: '3m',  target: 20 },  // sustained read load
    { duration: '1m',  target: 50 },  // peak
    { duration: '30s', target: 0  },  // ramp down
  ],
  thresholds: {
    // API reads should be well under 300 ms p99
    incident_list_ms:   ['p(99)<300'],
    incident_detail_ms: ['p(99)<200'],
    alert_list_ms:      ['p(99)<300'],
    // Liveness probe must always be fast
    'http_req_duration{name:health}': ['p(99)<20'],
    // Zero errors
    api_errors: ['rate<0.01'],
  },
};

const BASE_URL   = __ENV.BASE_URL   || 'http://localhost:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

// Cache a small pool of incident IDs to reuse across iterations
// (populated on first iteration per VU)
let incidentIDs = [];

function headers() {
  const h = {
    'Content-Type': 'application/json',
    // Unique per-VU IP so each VU gets its own rate-limit bucket (same as webhook test)
    'X-Forwarded-For': `10.1.${Math.floor(__VU / 256)}.${__VU % 256}`,
  };
  if (AUTH_TOKEN) {
    h['Authorization'] = `Bearer ${AUTH_TOKEN}`;
  }
  return h;
}

export default function () {
  // Health check — always fast, no auth needed
  group('health', function () {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'health' } });
    check(res, { 'health 200': (r) => r.status === 200 });
  });

  // List incidents — the heaviest read (JOIN + ORDER BY + LIMIT)
  group('incident list', function () {
    const res = http.get(`${BASE_URL}/api/v1/incidents`, {
      headers: headers(),
      tags: { name: 'incident_list' },
    });

    listLatency.add(res.timings.duration);

    const ok = check(res, {
      'list 200': (r) => r.status === 200,
      'list is array': (r) => {
        try {
          const body = JSON.parse(r.body);
          // Response can be array or {data: [...], total: N}
          return Array.isArray(body) || Array.isArray(body.data);
        } catch (_) {
          return false;
        }
      },
    });

    if (!ok) {
      errorRate.add(1);
      return;
    }
    errorRate.add(0);

    // Populate the ID pool from the list response (best-effort)
    if (incidentIDs.length === 0) {
      try {
        const body = JSON.parse(res.body);
        const items = Array.isArray(body) ? body : (body.data || []);
        incidentIDs = items.slice(0, 20).map((i) => i.id).filter(Boolean);
      } catch (_) {}
    }
  });

  // Incident detail — only if we have IDs
  if (incidentIDs.length > 0) {
    group('incident detail', function () {
      const id  = incidentIDs[Math.floor(Math.random() * incidentIDs.length)];
      const res = http.get(`${BASE_URL}/api/v1/incidents/${id}`, {
        headers: headers(),
        tags: { name: 'incident_detail' },
      });

      detailLatency.add(res.timings.duration);

      const ok = check(res, {
        'detail 200': (r) => r.status === 200,
        'detail has id': (r) => {
          try {
            return JSON.parse(r.body).id === id;
          } catch (_) {
            return false;
          }
        },
      });

      errorRate.add(ok ? 0 : 1);
    });
  }

  // Alert list
  group('alert list', function () {
    const res = http.get(`${BASE_URL}/api/v1/alerts`, {
      headers: headers(),
      tags: { name: 'alert_list' },
    });

    alertLatency.add(res.timings.duration);

    const ok = check(res, { 'alerts 200': (r) => r.status === 200 });
    errorRate.add(ok ? 0 : 1);
  });

  // Think time between page loads
  sleep(1 + Math.random() * 2);
}
