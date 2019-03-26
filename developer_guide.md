
# Developer Guide

## Git env

* Github fork into your own repo [example](https://github.com/jq/mpi-operator/)
* Git clone your own repo in local path
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
Create a symbolic link inside your GOPATH to the location you checked out the code

```sh
mkdir -p ${GOPATH}/src/github.com/kubeflow
ln -sf ${GIT_TRAINING} ${GOPATH}/src/github.com/kubeflow/mpi-operator
```

* GIT_TRAINING should be the location where you checked out https://github.com/kubeflow/mpi-operator

Resolve dependencies (if you don't have dep install, check how to do it [here](https://github.com/golang/dep))

Install dependencies by ```dep ensure```

add dependent library into [Gopkg.toml](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.toml)
and run `dep ensure`, make sure [Gopkg.lock](https://github.com/kubeflow/mpi-operator/blob/master/Gopkg.lock) file is correct.
Sometimes, it contains dependency to itself, you can try `rm -rf vendor/` or `rm Gopkg.lock` and redo `dep ensure` to fix it.
If it doesn't work, remove it from Gopkg.lock.
## Unit test
We use testify, see [pr](https://github.com/kubeflow/mpi-operator/pull/100) for example

## Go version

On ubuntu the default go package appears to be gccgo-go which has problems see [issue](https://github.com/golang/go/issues/15429) golang-go package is also really old so install from golang tarballs instead.

## Code Style
