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
REL_OSARCH="linux/amd64"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
RELEASE_VERSION?=v0.3.0
CONTROLLER_VERSION?=v2
BASE_IMAGE_SSH_PORT?=2222
IMG_BUILDER=docker
LD_FLAGS_V2=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
IMAGE_NAME?=mpioperator/mpi-operator
KUBEBUILDER_ASSETS_PATH := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))bin/kubebuilder/bin
KIND_VERSION=v0.17.0
# This kubectl version supports -k for kustomization.
KUBECTL_VERSION=v1.25.6
ENVTEST_K8S_VERSION=1.25.0
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS)

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
test: bin/envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -covermode atomic -coverprofile=profile.cov $(shell go list ./... | grep -v '/test/e2e')

# Only works with CONTROLLER_VERSION=v2
.PHONY: test_e2e
test_e2e: export TEST_MPI_OPERATOR_IMAGE = ${IMAGE_NAME}:${RELEASE_VERSION}
test_e2e: bin/kubectl kind images test_images dev_manifest
	go test -v ./test/e2e/...

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
	${IMG_BUILDER} build --build-arg VERSION=${CONTROLLER_VERSION} -t ${IMAGE_NAME}:${RELEASE_VERSION} .

.PHONY: test_images
test_images:
	${IMG_BUILDER} build --build-arg port=${BASE_IMAGE_SSH_PORT} -t mpioperator/base build/base
	${IMG_BUILDER} build -t mpioperator/openmpi build/base -f build/base/openmpi.Dockerfile
	${IMG_BUILDER} build -t mpioperator/openmpi-builder build/base -f build/base/openmpi-builder.Dockerfile
	${IMG_BUILDER} build -t mpioperator/mpi-pi:openmpi examples/v2beta1/pi
	${IMG_BUILDER} build -t mpioperator/intel build/base -f build/base/intel.Dockerfile
	${IMG_BUILDER} build -t mpioperator/intel-builder build/base -f build/base/intel-builder.Dockerfile
	${IMG_BUILDER} build -t mpioperator/mpi-pi:intel examples/v2beta1/pi -f examples/v2beta1/pi/intel.Dockerfile

.PHONY: tidy
tidy:
	go mod tidy -go 1.19

.PHONY: lint
lint: bin/golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run --new-from-rev=origin/master --go 1.19

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
	@GOBIN=$(PROJECT_DIR)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
bin/envtest: bin ## Download envtest-setup locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

bin/kubectl: bin
	curl -L -o $(PROJECT_DIR)/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/$(GOOS)/$(GOARCH)/kubectl
	chmod +x $(PROJECT_DIR)/bin/kubectl

.PHONY: kind
kind: bin
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/kind@${KIND_VERSION}

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(PROJECT_DIR)/bin/controller-gen
.PHONY: controller-gen
controller-gen: bin
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1

KUSTOMIZE = $(PROJECT_DIR)/bin/kustomize
.PHONY: kustomize
kustomize:
	@GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7
