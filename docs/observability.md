# Observability

The controller exposes controller-runtime metrics on `/metrics` and registers custom metrics for model workloads.

Custom metrics:

- `model_deployment_requests_total`
- `model_deployment_errors_total`
- `model_deployment_latency_ms`
- `model_deployment_tokens_per_second`
- `model_deployment_benchmark_duration_seconds`
- `model_deployment_ready_replicas`
- `model_deployment_status`
- `model_deployment_reconcile_errors_total`

Prometheus Operator:

```bash
kubectl apply -f config/prometheus/metrics_service.yaml
kubectl apply -f config/prometheus/servicemonitor.yaml
kubectl apply -f observability/prometheus/prometheus-rules.yaml
```

Grafana:

Import `observability/grafana/model-deployment-dashboard.json`.

OpenTelemetry:

`observability/otel/collector.yaml` is a design-ready collector sample. The current operator does not emit traces from inference providers.
