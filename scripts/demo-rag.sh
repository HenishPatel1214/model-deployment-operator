#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f examples/ollama-rag-qdrant.yaml
kubectl wait --for=condition=Available deployment/ollama-rag --timeout=180s || true
kubectl wait --for=condition=Available deployment/ollama-rag-qdrant --timeout=180s || true
kubectl get modeldeployment ollama-rag -o yaml
kubectl get deployment,svc,job -l app.kubernetes.io/instance=ollama-rag
