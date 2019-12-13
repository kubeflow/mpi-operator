# Roadmap of MPI Operator

This document provides a high-level overview of where MPI Operator will grow in future releases. See discussions in the original RFC [here](https://github.com/kubeflow/mpi-operator/pull/159).

## New Features / Enhancements

* Decouple the tight dependency on Open MPI and support other collective communication frameworks.
Related issue: [#12](https://github.com/kubeflow/mpi-operator/issues/12).
* Support new versions of MPI Operator in [kubeflow/manifests](https://github.com/kubeflow/manifests).
* Redesign different components of MPI Operator to support fault tolerant collective communication frameworks such as [caicloud/ftlib](https://github.com/caicloud/ftlib).
* Allow more flexible RBAC when `MPIJob`s so existing RBAC resources can be reused. Related issue: [#20](https://github.com/kubeflow/mpi-operator/issues/20).
* Support installation of MPI Operator via [Helm](https://github.com/helm/helm). Related issue: [#11](https://github.com/kubeflow/mpi-operator/issues/11).
* Support [Go modules](https://blog.golang.org/migrating-to-go-modules).
* Consider support launching framework-specific services such as [TensorBoard](https://www.tensorflow.org/tensorboard) and [Horovod Timeline](https://github.com/horovod/horovod#horovod-timeline). Since [tf-operator](https://github.com/kubeflow/tf-operator) already supports TensorBoard, we may want to consider moving this to [kubeflow/common](https://github.com/kubeflow/common) so it can be reused. Related issue: [#138](https://github.com/kubeflow/mpi-operator/issues/138).

## CI/CD

* Automate the process to publish images to Docker Hub whenever there's new release/commit. Related issue: [#93](https://github.com/kubeflow/mpi-operator/issues/93).
* Ensure new versions of `deploy/mpi-operator.yaml` are always compatible with [kubeflow/manifests](https://github.com/kubeflow/manifests).
* Add end-to-end tests via Kubeflow's testing infrastructure. Related issue: [#9](https://github.com/kubeflow/mpi-operator/issues/9).

## Bug Fixes

* Better statuses of launcher and worker pods. Related issues: [#90](https://github.com/kubeflow/mpi-operator/issues/90)
