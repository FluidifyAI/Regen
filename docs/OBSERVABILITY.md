# Observability: Collector, Backend Choice, Sampling, and Runbooks

This document covers the trace pipeline beyond what's in [OPERATIONS.md](./OPERATIONS.md#14-observability):
where traces go, how much of them you keep, and how to actually find one
when you need it. See `deploy/otel-collector/` for the collector and Tempo
configs referenced below, and [`docs/alerts/`](./alerts) for alert-source
setup (a different topic — that's inbound alerts, this is outbound traces).

---

## Architecture

```
Regen backend / frontend
        │  OTLP (gRPC :4317 / HTTP :4318)
        ▼
  OTel Collector
   ├─ redaction/pii   (defense in depth — see below)
   ├─ tail_sampling    (keep the interesting 100%, thin the rest)
   └─ batch
        │  OTLP
        ▼
   Trace backend (Tempo or SigNoz — see below)
```

Deploying the collector is optional. With no collector, traces still flow —
the app exports OTLP directly to whatever `OTEL_EXPORTER_OTLP_ENDPOINT`
points at — but you lose PII defense-in-depth and tail-based sampling. Both
matter enough at any real traffic volume that we recommend running the
collector for anything beyond local development.

---

## Backend choice: Grafana Tempo (recommended) or SigNoz

Both are OTLP-native, open-source, self-hostable. The docker-compose and
Helm chart additions in this repo default to **Tempo**; SigNoz is fully
supported as an alternative, just not bundled.

| | **Grafana Tempo** | **SigNoz** |
|---|---|---|
| Footprint | Single binary, one config file, object storage (or local disk) for blocks | ClickHouse + Query Service + Frontend + Alertmanager — several containers |
| Setup for self-hosters | `docker run grafana/tempo` + one YAML file | Full docker-compose stack (their own, ~8 containers) |
| UI | None built in — pairs with Grafana (already recommended for metrics in [OPERATIONS.md](./OPERATIONS.md)) | Own full UI: traces, metrics, logs, dashboards in one place |
| Query language | TraceQL | ClickHouse SQL under a UI |
| Metrics + logs story | Separate products (Mimir/Loki) you wire in yourself | Unified from day one |
| Best fit | Self-hosters who already run (or don't mind running) Grafana, and want the smallest possible footprint | Self-hosters who want one pane of glass and are fine with the heavier ClickHouse-backed stack |

**Recommendation: Tempo**, for the same reason this project defaults to
lightweight, single-purpose containers elsewhere (Postgres, Redis, one app
binary) rather than a heavier all-in-one platform. If you already run
SigNoz for other services, or want traces/metrics/logs unified without
also standing up Grafana, it's an equally valid choice — the collector
config in `deploy/otel-collector/otel-collector-config.yaml` only needs its
`exporters.otlp/tempo.endpoint` changed to point at SigNoz's OTLP receiver
instead.

---

## Sampling policy

**Tail-based, at the collector** (`processors.tail_sampling` in
`deploy/otel-collector/otel-collector-config.yaml`):

| Policy | Keeps |
|---|---|
| `keep-errors` | 100% of traces containing an error (`status_code = ERROR`) |
| `keep-incident-created` | 100% of traces where `incident.created = true` — set on the incident-create HTTP handler and the alert-triggered creation path |
| `keep-paged` | 100% of traces where `notification.paged = true` — set on `SendEscalationDM`, which starts its own root span since it has no ctx threaded from `EscalationEngine` |
| `sample-the-rest` | 1-5% (default 2%) of everything else, probabilistically |

Tail-based sampling can only happen at the collector: a single request
doesn't know whether its trace will end in an error until the trace is
over, and a trace spanning multiple services needs everything buffered in
one place to decide. This is why the collector is the thing doing this
policy, not the app.

**Head-based, in the app** (`OTEL_TRACES_SAMPLER` / `OTEL_TRACES_SAMPLER_ARG`):
standard OTel SDK env vars, honored automatically — no code change was
needed for this (`go.opentelemetry.io/otel/sdk/trace`'s own
`NewTracerProvider` applies them before any explicit option). This is the
escape hatch for environments running **without** a collector: if nothing
is doing tail-based sampling downstream, dial this down instead to bound
volume at the source. With a collector in place, leave this at the default
(`ParentBased(AlwaysSample)` — sample everything) and let the collector's
tail-based policy do the actual thinning; setting a head sample too would
mean traces get dropped before the collector ever sees whether they had an
error.

| Value | Effect |
|---|---|
| unset (default) | Sample everything — correct when a collector is doing tail-based sampling |
| `OTEL_TRACES_SAMPLER=traceidratio`, `OTEL_TRACES_SAMPLER_ARG=0.05` | Sample ~5% at the source — use only when there's no collector |
| `OTEL_TRACES_SAMPLER=always_off` | Disable tracing entirely without unsetting the OTLP endpoint |

---

## PII scrubbing (defense in depth)

The app itself never attaches incident titles, alert payloads, or email
addresses to spans (REG-5's observability Definition of Done; verified by
tests throughout REG-11/12/14). The collector's `redaction/pii` processor
is the *second* line: an explicit `allowed_keys` allow-list (so an
attribute nobody intended to emit gets dropped, not forwarded) plus a
`blocked_values` regex list (masks anything that looks like an email even
inside an allowed key's value).

This matters beyond defense in depth: **W13's GDPR/DSAR work cannot reach
into a trace store.** A right-to-erasure request can delete a row from
Postgres; it cannot selectively delete one user's data out of a trace
blob that's already been through tail-sampling and compaction. The scrubbing
processor is what keeps traces free of subject data in the first place, so
DSAR erasure never needs to.

The allow-list is a living document — see the comment at the top of
`deploy/otel-collector/otel-collector-config.yaml` for how to verify it
against what your actual library versions emit.

---

## Retention

**Default: 7 days** (`compactor.compaction.block_retention: 168h` in
`deploy/otel-collector/tempo.yaml`).

Traces are a subprocessor data flow, same as the point above — every day of
retention is a day of subject data sitting in a second store outside the
primary database's retention and erasure controls. Keep this short unless
you have a specific debugging need for longer history, and if you extend
it, make sure whoever owns the W13 DSAR process knows this store now needs
its own erasure handling too (it currently doesn't have any — short
retention is the whole mitigation).

---

## Deploying the collector

**Docker Compose** (opt-in profile — off by default):

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 \
  docker compose -f docker-compose.dev.yml --profile observability up
```

This starts `otel-collector` and `tempo` alongside the normal stack. Without
`--profile observability`, neither container starts and `api` never
attempts to export anywhere (`OTEL_EXPORTER_OTLP_ENDPOINT` stays empty).

**Helm** (`values.yaml`):

```yaml
tracing:
  otlpExporterEndpoint: "http://<release-name>-otel-collector:4317"
  collector:
    enabled: true
    otlpExporterEndpointDownstream: "tempo.observability.svc:4317" # your Tempo/SigNoz
    samplingPercentage: 2
```

The bundled chart template (`templates/otel-collector.yaml`) is a plain
Deployment/Service/ConfigMap — not the full upstream
`open-telemetry-collector` chart. If you need DaemonSet mode, host metrics,
or multi-tenant collection, use that chart directly and point
`tracing.otlpExporterEndpoint` at it instead of enabling the bundled one.

---

## Runbook: finding a trace

### From an incident ID

Every span on the incident create/update/lookup path carries an
`incident.id` attribute (the UUID, same as the API and UI use).

**Tempo (TraceQL):**
```
{ span.incident.id = "3fa85f64-5717-4562-b3fc-2c963f66afa6" }
```

**SigNoz:** Traces → filter by tag `incident.id` = the UUID.

If the incident was alert-triggered, the same attribute is on the
webhook-handling trace (`CreateIncidentFromAlert`/`CreateIncidentFromAlertWithGrouping`
tag it directly, since those already carry a real ctx); if it was created
via the API or UI, it's on the `POST /api/v1/incidents` trace, tagged
`incident.created = true` as well.

### From a customer complaint ("the incident page was slow")

1. **Narrow by time and route.** Ask when, roughly, and which page —
   `GET /api/v1/incidents/:id` for incident detail, `GET /api/v1/incidents`
   for the list.
2. **If they mention the frontend specifically**, the trace started in
   their browser (REG-14: every `/api/v1/*` fetch carries a `traceparent`
   header) and continues into this same backend trace — there's no
   separate "frontend trace" to find, it's the same one.
3. **Query by route + duration**, since you likely don't have a trace ID
   from a customer report:
   - **Tempo (TraceQL):** `{ span.http.request.method = "GET" && duration > 1s }`
     scoped to the relevant time window in the UI's time picker.
   - **SigNoz:** Traces → filter by `http.route` and a duration threshold,
     same time window.
4. **If nothing shows up**, check whether the trace was thinned by the
   probabilistic sampler — a slow-but-not-erroring, non-incident-creating,
   non-paging trace is exactly the ~95-98% of traffic the tail-sampling
   policy doesn't keep at 100%. Temporarily raising `sample-the-rest`'s
   `sampling_percentage` (or setting `OTEL_TRACES_SAMPLER_ARG` higher, with
   no collector) reproduces it going forward; it doesn't recover a trace
   that already wasn't kept.
