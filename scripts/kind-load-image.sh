#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER:-model-deployment}"
OPERATOR_IMG="${IMG:-ghcr.io/henishpatel1214/model-deployment-operator:latest}"
BENCHMARK_IMG="${BENCHMARK_IMG:-ghcr.io/henishpatel1214/model-deployment-benchmark:latest}"

docker build --target manager -t "${OPERATOR_IMG}" .
docker build --target benchmark -t "${BENCHMARK_IMG}" .
kind load docker-image "${OPERATOR_IMG}" --name "${CLUSTER_NAME}"
kind load docker-image "${BENCHMARK_IMG}" --name "${CLUSTER_NAME}"
