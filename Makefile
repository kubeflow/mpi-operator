BIN_DIR=_output/cmd/bin
REPO_PATH="github.com/kubeflow/mpi-operator"
REL_OSARCH="linux/amd64"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
RELEASE_VERSION=v0.2.2
IMG_BUILDER=docker
LD_FLAGS=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VERSION}'"
IMAGE_NAME?=kubeflow/mpi-operator

build: all

all: init mpi-operator.v1alpha1 mpi-operator.v1alpha2 kubectl-delivery

mpi-operator.v1alpha1:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v1alpha1 ./cmd/mpi-operator.v1alpha1/

mpi-operator.v1alpha2:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/mpi-operator.v1alpha2 ./cmd/mpi-operator.v1alpha2/

kubectl-delivery:
	go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/kubectl-delivery ./cmd/kubectl-delivery/

init:
	mkdir -p ${BIN_DIR}

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

clean:
	rm -fr ${BIN_DIR}

images:
	@echo "version: ${RELEASE_VERSION}"
	${IMG_BUILDER} build -t ${IMAGE_NAME}:${RELEASE_VERSION} .

.PHONY: clean
