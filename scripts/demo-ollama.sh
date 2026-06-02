#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f examples/ollama-benchmark.yaml
kubectl wait --for=condition=Available deployment/ollama-benchmark --timeout=180s || true
kubectl get modeldeployment ollama-benchmark -o yaml
kubectl get deployment,svc,hpa,job,configmap -l app.kubernetes.io/instance=ollama-benchmark

echo "Port-forward with:"
echo "kubectl port-forward svc/ollama-benchmark 11434:11434"
