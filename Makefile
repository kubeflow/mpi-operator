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
	cd v2 && go test -covermode atomic -coverprofile=profile.cov ./...

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

.PHONY: tidy
tidy:
	go mod tidy
	cd v2 && go mod tidy

GOLANGCI_LINT = ./bin/golangci-lint
bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) v1.29.0

.PHONY: lint
lint: bin/golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run --new-from-rev=origin/master
	cd v2 && $(GOLANGCI_LINT) run --new-from-rev=origin/master
