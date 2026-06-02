# API Reference

API group: `ai.platform.dev`

Version: `v1alpha1`

Kind: `ModelDeployment`

Important spec fields:

- `provider`: `ollama` or `vllm`
- `modelName`: model identifier passed into the workload
- `image`: inference container image
- `replicas`: desired replica count
- `resources`: CPU, memory, and optional GPU limit
- `service`: Service type and port
- `ingress`: optional host and TLS flag
- `autoscaling`: HPA settings
- `observability`: Prometheus and OpenTelemetry toggles
- `benchmark`: benchmark Job settings
- `rag`: optional Qdrant dependency
- `secrets`: registry, API token, and provider key secret references

Status fields:

- `phase`: `Pending`, `Running`, `Degraded`, `Scaling`, or `Failed`
- `readyReplicas`: ready Deployment replicas
- `endpoint`: cluster-local Service endpoint
- `lastBenchmark`: benchmark result parsed from the benchmark result ConfigMap
- `conditions`: Kubernetes-style conditions
