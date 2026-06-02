# Failure Handling

The controller reports failures through status conditions and Kubernetes events.

Image pull failures:

- Detection: Pod waiting reason `ImagePullBackOff` or `ErrImagePull`
- Phase: `Failed`
- Condition: `Ready=False`
- Recovery: fix image, registry, or secret reference

Crash loops:

- Detection: Pod waiting reason `CrashLoopBackOff`
- Phase: `Failed`
- Condition: `Ready=False`
- Recovery: inspect logs and fix model arguments or startup config

Out of memory:

- Detection: last termination reason `OOMKilled`
- Phase: `Failed`
- Condition: `Ready=False`
- Recovery: increase memory request/limit or reduce load

Benchmark failures:

- Detection: benchmark Job `.status.failed > 0`
- Condition: `BenchmarkReady=False`
- Recovery: inspect `kubectl logs job/<name>-benchmark`, validate Service reachability, and retry by deleting the Job

RAG dependency failures:

- Detection: Qdrant Deployment unavailable in normal Kubernetes status
- Recovery: inspect Qdrant pod logs, image pull state, resource limits, and NetworkPolicy
