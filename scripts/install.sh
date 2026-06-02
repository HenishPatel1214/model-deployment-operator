#!/usr/bin/env bash
set -euo pipefail

kubectl apply -k config/default
kubectl rollout status deployment/model-deployment-operator-controller-manager -n model-deployment-operator-system --timeout=120s
