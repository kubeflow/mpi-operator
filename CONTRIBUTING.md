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

We are currently using [dep](https://github.com/golang/dep) to download and install the dependencies. Please install `dep` and then run `dep ensure`. Any changes in dependencies should be modified in [Gopkg.toml](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.toml) and then [Gopkg.lock](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.lock) will be automatically generated via `dep ensure`.

## Run Unit Test

You can execute all the unit tests via `go test ./...`.

## Check Code Style

We use [gofmt](https://golang.org/cmd/gofmt/) to check and fix issues on code style. Please also check out [this wiki](https://github.com/golang/go/wiki/CodeReviewComments) for some additional instructions on code review.
