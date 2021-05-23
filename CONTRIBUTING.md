# Contributing Guide

Welcome to MPI Operator's contributing guide!

## Set Up Development Environment

If you haven't done so, please follow the instructions [here](https://help.github.com/en/github/getting-started-with-github/fork-a-repo) to fork and clone the repository, and then configure the remote repository for the repository you just cloned locally. Note that you'd probably want to clone your forked repository to be under your [`GOPATH`](https://github.com/golang/go/wiki/GOPATH), for example:

```bash
mkdir -p ${GOPATH}/src/github.com/kubeflow
cd ${GOPATH}/src/github.com/kubeflow
git clone https://github.com/${GITHUB_USER}/mpi-operator.git
```

## Install Dependencies

We use Go v1.13+ for development and use [Go Modules](https://blog.golang.org/using-go-modules) to download and install the dependencies.

## Run Unit Test

You can execute all the unit tests via `go test ./...`.

## Check Code Style

We use [golangci-lint](https://github.com/golangci/golangci-lint) to check issues on code style. Please also check out [this wiki](https://github.com/golang/go/wiki/CodeReviewComments) for some additional instructions on code review.

## Run

You have to build the image and deploy the standalone YAMLs in a cluster.

```shell
CONTROLLER_VERSION=v1 RELEASE_VERSION=latest make images
kubectl apply -k manifests/overlays/standalone
```

If you need to use a different registry, you can do:

```shell
IMAGE_NAME=example.com/mpi-operator CONTROLLER_VERSION=v1 RELEASE_VERSION=latest make images
```

Next, modify the line `newName` in `manifests/overlays/standalone/kustomization.yaml`
to match the image name. After pushing the image to the registry, you can apply
the YAMLs the same way as before.

To look at the controller's logs, you can do:

```shell
kubectl logs -n mpi-operator -f deployment/mpi-operator
```