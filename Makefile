# Copyright 2023 The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BIN_DIR=_output/cmd/bin
REPO_PATH="github.com/kubeflow/mpi-operator"
GitSHA=$(shell git rev-parse HEAD)
Date=$(shell date "+%Y-%m-%d %H:%M:%S")
RELEASE_VERSION?=v0.6.0
CONTROLLER_VERSION?=v2
BASE_IMAGE_SSH_PORT?=2222
IMG_BUILDER=docker
PLATFORMS ?= linux/amd64,linux/arm64,linux/ppc64le
INTEL_PLATFORMS ?= linux/amd64
MPICH_PLATFORMS ?= linux/amd64,linux/arm64
LD_FLAGS_V2=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
REGISTRY?=docker.io/mpioperator
IMAGE_NAME?=${REGISTRY}/mpi-operator
KUBEBUILDER_ASSETS_PATH := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))bin/kubebuilder/bin
KIND_VERSION=v0.18.0
HELM_VERSION=v3.11.2
# This kubectl version supports -k for kustomization.
KUBECTL_VERSION=v1.31.1
ENVTEST_K8S_VERSION=1.31.0
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS)
# Use go.mod go version as a single source of truth of scheduler-plugins version.
SCHEDULER_PLUGINS_VERSION?=$(shell go list -m -f "{{.Version}}" sigs.k8s.io/scheduler-plugins)
VOLCANO_SCHEDULER_VERSION?=$(shell go list -m -f "{{.Version}}" volcano.sh/apis)

CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=true"

build: all

all: ${BIN_DIR} fmt vet tidy lint test mpi-operator.v2

.PHONY: mpi-operator.v2
mpi-operator.v2:
	go build -ldflags ${LD_FLAGS_V2} -o ${BIN_DIR}/mpi-operator.v2 ./cmd/mpi-operator/

${BIN_DIR}:
	mkdir -p ${BIN_DIR}

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
test: bin/envtest scheduler-plugins-crd volcano-scheduler-crd
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -v -covermode atomic -coverprofile=profile.cov $(shell go list ./... | grep -v '/test/e2e')

# Only works with CONTROLLER_VERSION=v2
.PHONY: test_e2e
test_e2e: export TEST_MPI_OPERATOR_IMAGE=${IMAGE_NAME}:${RELEASE_VERSION}
test_e2e: export TEST_OPENMPI_IMAGE=${REGISTRY}/mpi-pi:${RELEASE_VERSION}-openmpi
test_e2e: export TEST_INTELMPI_IMAGE=${REGISTRY}/mpi-pi:${RELEASE_VERSION}-intel
test_e2e: export TEST_MPICH_IMAGE=${REGISTRY}/mpi-pi:${RELEASE_VERSION}-mpich
test_e2e: bin/kubectl kind helm images test_images dev_manifest scheduler-plugins-chart volcano-scheduler-deploy
	go test -timeout 20m -v ./test/e2e/...

.PHONY: dev_manifest
dev_manifest:
	# Use `~` instead of `/` because image name might contain `/`.
	sed -e "s~%IMAGE_NAME%~${IMAGE_NAME}~g" -e "s~%IMAGE_TAG%~${RELEASE_VERSION}~g" manifests/overlays/dev/kustomization.yaml.template > manifests/overlays/dev/kustomization.yaml

.PHONY: generate
generate:
	go generate ./pkg/... ./cmd/...
	hack/update-codegen.sh
	$(MAKE) manifest
	hack/python-sdk/gen-sdk.sh

.PHONY: verify-generate
verify-generate: generate
	git --no-pager diff --exit-code manifests/base deploy sdk pkg/apis pkg/client

.PHONY: clean
clean:
	rm -fr ${BIN_DIR}

.PHONY: images
images:
	@echo "VERSION: ${RELEASE_VERSION}"
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(PLATFORMS) --build-arg VERSION=${CONTROLLER_VERSION} --build-arg RELEASE_VERSION=${RELEASE_VERSION} -t ${IMAGE_NAME}:${RELEASE_VERSION} .

.PHONY: test_images
test_images:
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(PLATFORMS) --build-arg port=${BASE_IMAGE_SSH_PORT} -t ${REGISTRY}/base:${RELEASE_VERSION} build/base
	$(MAKE) -j3 test_images_openmpi test_images_intel test_images_mpich

.PHONY: test_images_openmpi
test_images_openmpi:
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/openmpi:${RELEASE_VERSION} build/base -f build/base/openmpi.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(PLATFORMS) -t ${REGISTRY}/openmpi-builder:${RELEASE_VERSION} build/base -f build/base/openmpi-builder.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/mpi-pi:${RELEASE_VERSION}-openmpi examples/v2beta1/pi

.PTHONY: test_images_intel
test_images_intel:
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(INTEL_PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/intel:${RELEASE_VERSION} build/base -f build/base/intel.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(INTEL_PLATFORMS) -t ${REGISTRY}/intel-builder:${RELEASE_VERSION} build/base -f build/base/intel-builder.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(INTEL_PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/mpi-pi:${RELEASE_VERSION}-intel examples/v2beta1/pi -f examples/v2beta1/pi/intel.Dockerfile

.PHOTNY: test_images_mpich
test_images_mpich:
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(MPICH_PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/mpich:${RELEASE_VERSION} build/base -f build/base/mpich.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(MPICH_PLATFORMS) -t ${REGISTRY}/mpich-builder:${RELEASE_VERSION} build/base -f build/base/mpich-builder.Dockerfile
	${IMG_BUILDER} build $(BUILD_ARGS) --platform $(MPICH_PLATFORMS) --build-arg BASE_LABEL=${RELEASE_VERSION} -t ${REGISTRY}/mpi-pi:${RELEASE_VERSION}-mpich examples/v2beta1/pi -f examples/v2beta1/pi/mpich.Dockerfile

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: lint
lint: bin/golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

# Generate deploy/v2beta1/mpi-operator.yaml
manifest: kustomize crd
	hack/generate-manifest.sh $(KUSTOMIZE)

# Generate CRD
crd: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=manifests/base

.PHONY: bin
bin:
	mkdir -p $(PROJECT_DIR)/bin

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
.PHONY: bin/golangci-lint
bin/golangci-lint: bin
	@GOBIN=$(PROJECT_DIR)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
bin/envtest: bin ## Download envtest-setup locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

bin/kubectl: bin
	curl -L -o $(PROJECT_DIR)/bin/kubectl https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/$(GOOS)/$(GOARCH)/kubectl
	chmod +x $(PROJECT_DIR)/bin/kubectl

.PHONY: kind
kind: bin
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/kind@${KIND_VERSION}

.PHONY: helm
helm: bin
	@GOBIN=$(PROJECT_DIR)/bin go install helm.sh/helm/v3/cmd/helm@${HELM_VERSION}

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(PROJECT_DIR)/bin/controller-gen
.PHONY: controller-gen
controller-gen: bin
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.4

KUSTOMIZE = $(PROJECT_DIR)/bin/kustomize
.PHONY: kustomize
kustomize:
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7

# The build via `go install` will fail with the following err:
# > The go.mod file for the module providing named packages contains one or
#   more replace directives. It must not contain directives that would cause
#   it to be interpreted differently than if it were the main module.
# However, we can ignore the above error since it is only necessary to download the manifests.
.PHONY: scheduler-plugins
scheduler-plugins:
	-@GOPATH=/tmp go install sigs.k8s.io/scheduler-plugins/cmd/controller@$(SCHEDULER_PLUGINS_VERSION)

.PHONY: scheduler-plugins-crd
scheduler-plugins-crd: scheduler-plugins
	mkdir -p $(PROJECT_DIR)/dep-crds/scheduler-plugins/
	cp -f /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/coscheduling/* $(PROJECT_DIR)/dep-crds/scheduler-plugins

.PHONY: scheduler-plugins-chart
scheduler-plugins-chart: scheduler-plugins-crd
	mkdir -p $(PROJECT_DIR)/dep-manifests/scheduler-plugins/
	cp -rf /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/install/charts/as-a-second-scheduler/* $(PROJECT_DIR)/dep-manifests/scheduler-plugins
	mkdir -p $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds
	cp -f /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/appgroup/crd.yaml $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds/appgroup.diktyo.x-k8s.io_appgroups.yaml
	cp -f /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/networktopology/crd.yaml $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds/networktopology.diktyo.x-k8s.io_networktopologies.yaml
	cp -f /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/capacityscheduling/crd.yaml $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds/scheduling.x-k8s.io_elasticquotas.yaml
	cp -f $(PROJECT_DIR)/dep-crds/scheduler-plugins/crd.yaml $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds/scheduling.x-k8s.io_podgroups.yaml
	cp -f /tmp/pkg/mod/sigs.k8s.io/scheduler-plugins@$(SCHEDULER_PLUGINS_VERSION)/manifests/noderesourcetopology/crd.yaml $(PROJECT_DIR)/dep-manifests/scheduler-plugins/crds/topology.node.k8s.io_noderesourcetopologies.yaml
	chmod -R 760 $(PROJECT_DIR)/dep-manifests/scheduler-plugins

.PHONY: volcano-scheduler
volcano-scheduler:
	rm -rf /tmp/volcano.sh/volcano
	git clone -b $(VOLCANO_SCHEDULER_VERSION) --depth 1 https://github.com/volcano-sh/volcano /tmp/volcano.sh/volcano

.PHONY: volcano-scheduler-crd
volcano-scheduler-crd: volcano-scheduler
	mkdir -p $(PROJECT_DIR)/dep-crds/volcano-scheduler/
	cp -f /tmp/volcano.sh/volcano/config/crd/volcano/bases/* $(PROJECT_DIR)/dep-crds/volcano-scheduler

.PHONY: volcano-scheduler-deploy
volcano-scheduler-deploy: volcano-scheduler-crd
	mkdir -p $(PROJECT_DIR)/dep-manifests/volcano-scheduler/
	cp -f /tmp/volcano.sh/volcano/installer/volcano-development.yaml $(PROJECT_DIR)/dep-manifests/volcano-scheduler/
