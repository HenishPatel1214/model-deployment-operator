SHELL := /usr/bin/env bash

IMG ?= ghcr.io/henishpatel1214/model-deployment-operator:latest
BENCHMARK_IMG ?= ghcr.io/henishpatel1214/model-deployment-benchmark:latest
KIND_CLUSTER ?= model-deployment

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

.PHONY: all
all: fmt vet test build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l . | grep -E '\.go$$')" || (gofmt -l . | grep -E '\.go$$' && exit 1)

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test: manifests envtest
	KUBEBUILDER_ASSETS="$$( $(ENVTEST) use 1.30.0 --bin-dir $(LOCALBIN) -p path )" go test ./... -coverprofile coverage.out

.PHONY: test-fast
test-fast:
	go test ./...

.PHONY: build
build:
	go build -o bin/manager ./cmd/manager
	go build -o bin/benchmark ./cmd/benchmark

.PHONY: run
run: manifests
	go run ./cmd/manager/main.go

.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:allowDangerousTypes=true paths="./..." output:crd:artifacts:config=config/crd/bases output:rbac:artifacts:config=config/rbac
	cp config/crd/bases/ai.platform.dev_modeldeployments.yaml charts/model-deployment-operator/crds/ai.platform.dev_modeldeployments.yaml

.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: docker-build
docker-build:
	docker build --target manager -t $(IMG) .
	docker build --target benchmark -t $(BENCHMARK_IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)
	docker push $(BENCHMARK_IMG)

.PHONY: install
install: manifests
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall:
	kubectl delete -f config/crd/bases --ignore-not-found=true

.PHONY: deploy
deploy: manifests
	kubectl apply -k config/default

.PHONY: undeploy
undeploy:
	kubectl delete -k config/default --ignore-not-found=true

.PHONY: helm-lint
helm-lint:
	helm lint charts/model-deployment-operator

.PHONY: controller-gen
controller-gen:
	@test -s $(CONTROLLER_GEN) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.21.0

.PHONY: envtest
envtest:
	@test -s $(ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.19

$(ENVTEST): envtest
