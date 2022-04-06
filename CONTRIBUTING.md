# Kubeflow Contributor Guide

Welcome to the Kubeflow/MPI-Operator project! We'd love to accept your patches and 
contributions to this project. Please read the 
[contributor's guide in our docs](https://www.kubeflow.org/docs/about/contributing/).

The contributor's guide

* shows you where to find the Contributor License Agreement (CLA) that you need 
  to sign,
* helps you get started with your first contribution to Kubeflow,
* and describes the pull request and review workflow in detail, including the
  OWNERS files and automated workflow tool.

## Set Up Development Environment

If you haven't done so, please follow the instructions [here](https://help.github.com/en/github/getting-started-with-github/fork-a-repo) to fork and clone the repository, and then configure the remote repository for the repository you just cloned locally. Note that you'd probably want to clone your forked repository to be under your [`GOPATH`](https://github.com/golang/go/wiki/GOPATH), for example:

```bash
mkdir -p ${GOPATH}/src/github.com/kubeflow
cd ${GOPATH}/src/github.com/kubeflow
git clone https://github.com/${GITHUB_USER}/mpi-operator.git
```

## Install Dependencies

We use Go v1.15+ for development and use [Go Modules](https://blog.golang.org/using-go-modules) to download and install the dependencies.

## Controller versions

The main module `github.com/kubeflow/mpi-operator` contains the code of the legacy
controller `v1`.

The newest iteration of the controller is in the module `github.com/kubeflow/mpi-operator/v2`.

## Run tests

### Unit and integration tests

You can execute all the unit and integration tests via `make test`.

If you only which to run the tests for the v2 controller, you can run `make test_v2`.

You can find the unit tests in the same folders as the functional code.

You can find the integration tests in a separate directory, `v2/test/integration`.
Integration tests make use of a real kube-apiserver to test the interaction of
the controller with a real Kubernetes API. In these tests, other components
are not running, including `kubelet` or `kube-controller-manager`.

Consider adding an integration test if your feature makes new API calls.

### E2E tests

E2E tests run against a real cluster. In our tests, we create a cluster using
[kind](https://kind.sigs.k8s.io/docs/user/quick-start/).

You can run the tests with `make test_e2e`.

If desired, you can run the tests against any existing cluster. Just make sure
that credentials for the cluster are present in `${HOME}/.kube/config` and run:

```bash
USE_EXISTING_CLUSTER=true make test_e2e
```

## Check Code Style

We use [golangci-lint](https://github.com/golangci/golangci-lint) to check issues on code style.
Please also check out [this wiki](https://github.com/golang/go/wiki/CodeReviewComments) for some additional instructions on code review.

You can run formatter and linter with:

```bash
make fmt lint
```

## Run

You have to build the image and deploy the standalone YAMLs in a cluster.

```bash
make images dev_manifest
kubectl apply -k manifests/overlays/dev
```

The image comes bundled with all the controller versions. For example, you can
find the v1 controller binary at `/opt/mpi-operator.v1`.

If you need to use a different registry, or a different tag, you can do:

```bash
make IMAGE_NAME=example.com/mpi-operator RELEASE_VERSION=dev make images dev_manifest
```

To look at the controller's logs, you can do:

```shell
kubectl logs -n mpi-operator -f deployment/mpi-operator
```