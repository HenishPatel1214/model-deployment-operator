#!/usr/bin/env bash
set -euo pipefail

kubectl delete -f examples/ollama-basic.yaml --ignore-not-found=true
kubectl delete -f examples/ollama-autoscaling.yaml --ignore-not-found=true
kubectl delete -f examples/ollama-ingress.yaml --ignore-not-found=true
kubectl delete -f examples/ollama-benchmark.yaml --ignore-not-found=true
kubectl delete -f examples/ollama-rag-qdrant.yaml --ignore-not-found=true
kubectl delete -f examples/vllm-basic.yaml --ignore-not-found=true
kubectl delete -k config/default --ignore-not-found=true
