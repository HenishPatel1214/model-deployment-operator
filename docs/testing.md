# Testing

Run the standard checks:

```bash
make fmt-check
make vet
make test
make build
make docker-build
make helm-lint
```

`make test` installs envtest assets through `setup-envtest` and runs controller tests against a real API server and etcd.

Covered behavior:

- Deployment creation
- Service creation
- Ingress creation
- HPA creation
- Benchmark Job creation
- Benchmark result ConfigMap and scoped RBAC creation
- Qdrant Deployment and Service creation
- Cost report ConfigMap creation
- Finalizer addition
- Status update
- Deletion cleanup
