# Demo Script

Use this as a short walkthrough for a recorded portfolio demo.

```bash
scripts/kind-create.sh
scripts/kind-load-image.sh
scripts/install.sh
kubectl get pods -n model-deployment-operator-system
kubectl apply -f examples/ollama-benchmark.yaml
kubectl get modeldeployments
kubectl get deployment,svc,hpa,job,configmap -l app.kubernetes.io/instance=ollama-benchmark
kubectl logs job/ollama-benchmark-benchmark
kubectl get configmap ollama-benchmark-benchmark-result -o yaml
kubectl get modeldeployment ollama-benchmark -o yaml
kubectl delete modeldeployment ollama-benchmark
kubectl get deployment,svc,hpa,job,configmap -l app.kubernetes.io/instance=ollama-benchmark
```

Narration points:

- One CR creates the complete model-serving surface.
- Status reports phase, endpoint, ready replicas, and benchmark results.
- Finalizers verify cleanup of operator-owned resources.
- Helm and Argo CD make the operator GitOps-ready.
