# Architecture

The operator follows a standard controller-runtime reconciliation loop:

1. Watch `ModelDeployment` objects.
2. Add a finalizer.
3. Build desired child resources from the spec.
4. Create or update child resources with owner references.
5. Read Deployment, Pod, HPA, Job, and ConfigMap state.
6. Update `status.phase`, `status.readyReplicas`, `status.endpoint`, `status.lastBenchmark`, and conditions.
7. On deletion, verify cleanup before removing the finalizer.

Core child resources:

- Inference Deployment and Service
- Optional Ingress
- Optional HPA
- Optional benchmark Job with result ConfigMap and scoped RBAC
- Optional Qdrant Deployment and Service for RAG-ready mode
- Runtime and cost report ConfigMaps

The operator uses owner references for garbage collection and a finalizer for explicit cleanup verification.
