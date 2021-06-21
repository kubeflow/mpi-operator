BIN_DIR=../_output/cmd/bin
REPO_PATH="github.com/kubeflow/mpi-operator/v2"
REL_OSARCH="linux/amd64"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
RELEASE_VERSION?=v2.0.0
IMG_BUILDER=docker
LD_FLAGS=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
IMAGE_NAME?=kubeflow/mpi-operator

build: all

all: init fmt tidy lint test mpi-operator

mpi-operator:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v2 ./cmd/mpi-operator/

init:
	mkdir -p ${BIN_DIR}

fmt:
	go fmt ./...

test: 
	go test -covermode atomic -coverprofile=profile.cov ./...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...
	openapi-gen --input-dirs github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/util/intstr,k8s.io/apimachinery/pkg/version,github.com/kubeflow/common/pkg/apis/common/v1 --output-package github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2 --go-header-file ../hack/boilerplate/boilerplate.go.txt

clean:
	rm -fr ${BIN_DIR}

image:
	@echo "version: ${RELEASE_VERSION}"
	${IMG_BUILDER} build -t ${IMAGE_NAME}:${RELEASE_VERSION} .. -f ../Dockerfile.v2

tidy:
	go mod tidy

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
bin/golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) v1.29.0 ;\
	}

lint: bin/golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run --new-from-rev=origin/master

.PHONY: clean
