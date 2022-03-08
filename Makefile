BIN_DIR=_output/cmd/bin
REPO_PATH="github.com/kubeflow/mpi-operator"
REL_OSARCH="linux/amd64"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
RELEASE_VERSION?=v0.3.0
CONTROLLER_VERSION?=v2
BASE_IMAGE_SSH_PORT?=2222
IMG_BUILDER=docker
LD_FLAGS=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
LD_FLAGS_V2=" \
    -X '${REPO_PATH}/v2/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/v2/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/v2/pkg/version.Version=${RELEASE_VERSION}'"
IMAGE_NAME?=mpioperator/mpi-operator
KUBEBUILDER_ASSETS_PATH := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))bin/kubebuilder/bin
KIND_VERSION=v0.11.1
# This kubectl version supports -k for kustomization.
KUBECTL_VERSION=v1.21.4

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

build: all

all: ${BIN_DIR} fmt tidy lint test mpi-operator.v1 mpi-operator.v2 kubectl-delivery

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
	cd v2 && go test -covermode atomic -coverprofile=profile.cov ./...

# Only works with CONTROLLER_VERSION=v2
.PHONY: test_e2e
test_e2e: export TEST_MPI_OPERATOR_IMAGE = ${IMAGE_NAME}:${RELEASE_VERSION}
test_e2e: bin/kubectl kind images test_images dev_manifest
	cd v2 && go test -tags e2e ./test/e2e/...

.PHONY: dev_manifest
dev_manifest:
	# Use `~` instead of `/` because image name might contain `/`.
	sed -e "s~%IMAGE_NAME%~${IMAGE_NAME}~g" -e "s~%IMAGE_TAG%~${RELEASE_VERSION}~g" manifests/overlays/dev/kustomization.yaml.template > manifests/overlays/dev/kustomization.yaml

.PHONY: generate
generate:
	go generate ./pkg/... ./cmd/...
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

# Generate CRD
crd: controller-gen
	cd v2 && $(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=crd

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(PROJECT_DIR)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.1)

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef