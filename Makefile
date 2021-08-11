BIN_DIR=_output/cmd/bin
REPO_PATH="github.com/kubeflow/mpi-operator"
REL_OSARCH="linux/amd64"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
RELEASE_VERSION?=v0.2.2
CONTROLLER_VERSION?=v1alpha2
IMG_BUILDER=docker
LD_FLAGS=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
LD_FLAGS_V2=" \
    -X '${REPO_PATH}/v2/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/v2/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/v2/pkg/version.Version=${RELEASE_VERSION}'"
IMAGE_NAME?=kubeflow/mpi-operator
KUBEBUILDER_ASSETS_PATH := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))bin/kubebuilder/bin
KIND_VERSION=v0.11.1
# This kubectl version supports -k for kustomization.
KUBECTL_VERSION=v1.21.4

build: all

all: ${BIN_DIR} fmt tidy lint test mpi-operator.v1alpha1 mpi-operator.v1alpha2 mpi-operator.v1 mpi-operator.v2 kubectl-delivery

.PHONY: mpi-operator.v1alpha1
mpi-operator.v1alpha1:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v1alpha1 ./cmd/mpi-operator.v1alpha1/

.PHONY: mpi-operator.v1alpha2
mpi-operator.v1alpha2:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v1alpha2 ./cmd/mpi-operator.v1alpha2/

.PHONY: mpi-operator.v1
mpi-operator.v1:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v1 ./cmd/mpi-operator.v1/

.PHONY: mpi-operator.v2
mpi-operator.v2:
	cd v2 && \
	go build -ldflags ${LD_FLAGS_V2} -o ../${BIN_DIR}/mpi-operator.v2 ./cmd/mpi-operator/

.PHONY: kubectl-delivery
kubectl-delivery:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/kubectl-delivery ./cmd/kubectl-delivery/

${BIN_DIR}:
	mkdir -p ${BIN_DIR}

.PHONY: fmt
fmt:
	go fmt ./...
	cd v2 && go fmt ./...

.PHONY: test
test:
	go test -covermode atomic -coverprofile=profile.cov ./...
	@make test_v2

.PHONY: test_v2
test_v2: export KUBEBUILDER_ASSETS = ${KUBEBUILDER_ASSETS_PATH}
test_v2: bin/kubebuilder
	cd v2 && go test -covermode atomic -coverprofile=profile.cov ./cmd/... ./pkg/... ./test/integration/...

# Only works with CONTROLLER_VERSION=v2
.PHONY: test_e2e
test_e2e: export TEST_MPI_OPERATOR_IMAGE = ${IMAGE_NAME}:${RELEASE_VERSION}
test_e2e: bin/kubectl kind images test_images dev_manifest
	cd v2 && go test ./test/e2e/...

.PHONY: dev_manifest
dev_manifest:
	# Use `~` instead of `/` because image name might contain `/`.
	sed -e "s~%IMAGE_NAME%~${IMAGE_NAME}~g" -e "s~%IMAGE_TAG%~${RELEASE_VERSION}~g" manifests/overlays/dev/kustomization.yaml.template > manifests/overlays/dev/kustomization.yaml

.PHONY: generate
generate:
	go generate ./pkg/... ./cmd/...
	@echo "Generating OpenAPI specification for v1alpha2..."
	openapi-gen --input-dirs github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/util/intstr,k8s.io/apimachinery/pkg/version,github.com/kubeflow/common/pkg/apis/common/v1 --output-package github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2 --go-header-file hack/boilerplate/boilerplate.go.txt
	@echo "Generating OpenAPI specification for v1..."
	openapi-gen --input-dirs github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/util/intstr,k8s.io/apimachinery/pkg/version,github.com/kubeflow/common/pkg/apis/common/v1 --output-package github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1 --go-header-file hack/boilerplate/boilerplate.go.txt

.PHONY: generate_v2
generate_v2:
	cd v2 && \
	go generate ./pkg/... ./cmd/... && \
	openapi-gen --input-dirs github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/util/intstr,k8s.io/apimachinery/pkg/version,github.com/kubeflow/common/pkg/apis/common/v1 --output-package github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2 --go-header-file ../hack/boilerplate/boilerplate.go.txt

.PHONY: clean
clean:
	rm -fr ${BIN_DIR}

.PHONY: images
images:
	@echo "version: ${RELEASE_VERSION}"
	${IMG_BUILDER} build --build-arg version=${CONTROLLER_VERSION} -t ${IMAGE_NAME}:${RELEASE_VERSION} .

.PHONY: test_images
test_images:
	${IMG_BUILDER} build -t kubeflow/mpi-pi:openmpi examples/pi

.PHONY: tidy
tidy:
	go mod tidy
	cd v2 && go mod tidy

GOLANGCI_LINT = ./bin/golangci-lint
bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) v1.29.0

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
K8S_VERSION := "1.19.2"
bin/kubebuilder:
	curl -sSLo envtest-bins.tar.gz "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-${K8S_VERSION}-${GOOS}-${GOARCH}.tar.gz"
	mkdir -p bin/kubebuilder
	tar -C bin/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
	rm envtest-bins.tar.gz

bin/kubectl:
	mkdir -p bin
	curl -L -o bin/kubectl https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl
	chmod +x bin/kubectl

.PHONY: lint
lint: bin/golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run --new-from-rev=origin/master
	cd v2 && ../$(GOLANGCI_LINT) run --new-from-rev=origin/master

.PHONY: kind
kind:
	go install sigs.k8s.io/kind@${KIND_VERSION}