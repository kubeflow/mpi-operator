
# Developer Guide

## Git env

* Github fork into your own repo [example](https://github.com/jq/mpi-operator/)
* Git clone your own repo in local path
```sh
    mkdir -p ${GOPATH}/src/github.com/kubeflow
    cd ${GOPATH}/src/github.com/kubeflow
    git clone https://github.com/${GITHUB_USER}/mpi-operator.git
```
* cd into the path and `git remote add upstream git@github.com:kubeflow/mpi-operator.git`
* git checkout from upstream, [example of alias](https://github.com/jq/mac/blob/e3b84d39cfdf37e8f9e0440d7a5bd98b992cf55e/git.sh#L70)
```
	git checkout -t upstream/master -b your_local_branch
	git pull --rebase
```
* from now on `git pull --rebase` with upstream and no ugly merge node in the git repo
* `git push origin your_branch` push to your repo
* create merge request in github from your repo

## Go Dev

install [dep](https://github.com/golang/dep) and then run `dep ensure`

add dependent library into [Gopkg.toml](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.toml)
and run `dep ensure`, and make sure [Gopkg.lock](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.lock) file is correct.

## Unit test
run `go test ./...` as what's done in Travis

## Ubuntu

For ubuntu follow the [install guide](https://github.com/golang/go/wiki/Ubuntu)

## [Code Style](https://github.com/golang/go/wiki/CodeReviewComments)
