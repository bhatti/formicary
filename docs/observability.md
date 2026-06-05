# Observability: Distributed Tracing

Formicary emits OpenTelemetry (OTel) traces that span the full request path, from the API or webhook entry point through the scheduler, job supervisor, task dispatcher, and down into the ant worker. Trace context is propagated across queue messages, so every hop appears as a child span in your observability backend.

---

## Enabling Tracing

Add a `tracing` block under `common` in your `formicary-queen.yaml` and `formicary-ant.yaml`:

```yaml
common:
  tracing:
    enabled: true
    endpoint: http://localhost:4318   # OTLP/HTTP endpoint
    sample_ratio: 1.0                 # 1.0 = trace everything; 0.1 = 10% in production
```

The endpoint must accept OTLP over HTTP (port 4318 for Jaeger ≥ 1.35, Grafana Tempo, Honeycomb, etc.).

---

## Quick Start with Jaeger

```bash
# Run Jaeger all-in-one (accepts OTLP/HTTP on 4318, serves UI on 16686)
docker run --rm -p 4318:4318 -p 16686:16686 \
  jaegertracing/all-in-one:latest

# Start Formicary with tracing enabled
cat >> formicary-queen.yaml <<'EOF'
common:
  tracing:
    enabled: true
    endpoint: http://localhost:4318
    sample_ratio: 1.0
EOF

./formicary-queen --config formicary-queen.yaml

# Open the Jaeger UI
open http://localhost:16686
```

Submit a job and search for service `formicary-queen` in the Jaeger UI. You will see a trace tree like:

```
HTTP POST /api/jobs/requests            [formicary.http]
  └─ job.supervise                      [formicary.queen]
       └─ task.dispatch                 [formicary.queen]  → queue message
            └─ task.receive             [formicary.ant]    ← queue message
                 ├─ task.pre_process    [formicary.ant]
                 ├─ task.execute_script [formicary.ant]
                 └─ task.post_process   [formicary.ant]
```

---

## Span Catalogue

| Tracer | Span | Attributes |
|--------|------|-----------|
| `formicary.http` | `HTTP {METHOD} {path}` | `http.request.method`, `url.path`, `http.response.status_code` |
| `formicary.grpc` | gRPC method path | `rpc.system=grpc`, `rpc.method`, `rpc.grpc.status_code` |
| `formicary.queen` | `job.launch` | `job.type`, `job.request_id`, `job.execution_id` |
| `formicary.queen` | `job.supervise` | `job.type`, `job.request_id`, `job.execution_id` |
| `formicary.queen` | `task.dispatch` | `task.type`, `job.type`, `job.request_id`, `messaging.destination` |
| `formicary.ant` | `task.receive` | `task.type`, `job.type`, `job.request_id`, `messaging.correlation_id` |
| `formicary.ant` | `task.execute` | `task.type`, `job.type`, `job.request_id`, `executor.method` |
| `formicary.ant` | `task.pre_process` | — |
| `formicary.ant` | `task.execute_script` | `script.before_count`, `script.main_count` |
| `formicary.ant` | `task.post_process` | — |
| `formicary.trigger` | `trigger.evaluate` | `trigger.type`, `trigger.name`, `job.type`, `trigger.rate_limited` |
| `formicary.trigger` | `trigger.submit_job` | `job.type`, `trigger.name`, `trigger.dedup_key` |
| `formicary.trigger` | `trigger.webhook` | `trigger.name`, `job.type`, `http.method` |
| `formicary.trigger` | `trigger.s3_poll` | `trigger.name`, `job.type`, `s3.bucket`, `s3.prefix` |
| `formicary.trigger` | `trigger.queue_message` | `trigger.name`, `job.type`, `queue.topic` |

Errors are recorded via `span.RecordError()` and set the span status to `ERROR`.

---

## Context Propagation

Trace context flows across process and queue boundaries automatically:

- **HTTP requests**: W3C `traceparent` / `tracestate` headers extracted by the Echo middleware.
- **gRPC calls**: `traceparent` injected into and extracted from gRPC metadata.
- **Queue messages**: `traceparent` injected into message properties by the queen (task dispatch, job launch) and extracted by the ant (task receive). The same mechanism is used for all queue providers (Redis streams, Kafka, Pulsar, channels).

This means a trace started by an external system (e.g. GitHub webhook → `POST /api/triggers/...`) propagates all the way to the ant worker executing the job, giving a single end-to-end trace in your observability backend.

---

## Configuration Reference

| Key | Default | Description |
|-----|---------|-------------|
| `common.tracing.enabled` | `false` | Enable OTel export. |
| `common.tracing.endpoint` | `http://localhost:4318` | OTLP/HTTP exporter endpoint. |
| `common.tracing.sample_ratio` | `1.0` | Fraction of traces to export. Reduce to `0.01`–`0.1` in high-throughput production. |

The same block is valid in both `formicary-queen.yaml` and `formicary-ant.yaml`. Each process registers itself under a different service name:

| Process | Service name in traces |
|---------|----------------------|
| Queen server | `formicary-queen` |
| Standalone ant | `formicary-ant` |
| Embedded ant | `formicary-ant` |

---

## Tips

- **Sample in production.** A `sample_ratio` of `0.05`–`0.1` is typical for high-throughput deployments. Tail-based sampling in Grafana Tempo or the OTel Collector can keep 100% of error traces while dropping healthy ones.
- **Correlate with logs.** Logrus structured log fields (`RequestID`, `JobType`, `TaskType`) match OTel span attributes, making it easy to jump from a log line to the corresponding trace in Jaeger or Grafana.
- **No code changes needed.** The tracing middleware is installed globally. Disabling `enabled: false` is a zero-cost no-op — the global OTel provider uses a no-op exporter and all `Start/End` calls compile away to nearly nothing.
