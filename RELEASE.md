# MPI Operator Releases

## Release v0.6.0
* Features:
  * Support ManagedBy feature (`.spec.runPolicy.managedBy`) inspired by batch/v1 Job.
    * This allows us to dispatch MPIJobs to the multiple clusters powered by Kueue's MultiKueue. (#650, @mszadkow)
* Clean ups:
  * Upgrade k8s libraries to v1.31 (#664, @ArangoGutierrez)
  * Upgrade debian version to bookworm and MPI versions are upgraded in the following: (#661, @tenzen-y)
    * OpenMPI: v4.1.0 -> v4.1.4
    * MPICH: 3.4.1 -> 4.0.2

## Release v0.5.0
* Features:
  * Add support for MPICH (#562, @sheevy)
  * Field runLauncherAsWorker allows to add the launcher pod into the hostfile as a worker (#612, @kuizhiqing)
  * Add PodGroup minResources calculation for volcano integration (#566, @lowang-bh)
* Bug fixes:
  * Fix panic when using PodGroups and PriorityClasses (#561, @tenzen-y)
  * Fix installation of mpijob Python module (#579, @vsoch)
  * Fix hostfile when jobs in different namespaces have the same name (#622, @kuizhiqing)
* Clean ups:
  * Upgrade k8s libraries to v1.29 (#633, @tenzen-y)
  * Fail the mpi-operator binary if access to API is denied (#619, @emsixteeen)

## Release v0.4.0
* Breaking changes
  * Removed v1 operator. If you want to use MPIJob v1, you can use the training-operator.
* Support for suspending semantics. Third party controllers can leverage the suspend field to implement queuing and preemption for an MPIJob.
* Support for the coscheduling plugins of the scheduler-plugins. 
* The operator supports multi-architecture (amd64, aarch64, and ppc64le).
* Bug fixes
  * Fix support for elastic Horovod.

## Release v0.3.0

* Scalability improvements
  * Worker start up no longer issues requests to kube-apiserver.
  * Dropped kubectl-delivery init container, reducing stress on kube-apiserver.
* Support for Intel MPI.
* Support for `runPolicy` (`ttlSecondsAfterFinish`, `activeDeadlineSeconds`, `backoffLimit`)
  by using a k8s Job for the launcher.
* Samples for plain MPI applications.
* Production readiness improvements:
  * Increased coverage throughout unit, integration and E2E tests.
  * More robust API validation.
  * Revisited v2beta1 MPIJob API.
  * Using fully-qualified label names, in consistency with other kubeflow operators.

## Release v0.2.3

### Enhancements

* Added support for RH OCP4.1 and RH OCP4.2
* Added additional installation methods
   * Using kustomize and [kubeflow/manifests](https://github.com/kubeflow/manifests)
   * Using [Helm Chart](https://github.com/kubeflow/mpi-operator/tree/master/hack/helm/mpi-operator)
* Added support for Go Modules and removed vendor directories
* Added default ephemeral storage for init container
* Overwrite NVIDIA env vars to avoid using GPUs on launcher
* Added health check and callbacks around various leader election phases
* Honor user-specified worker command
* Exposed main container name as a configurable field
* Added RunPolicy to MPIJobSpec that reuses [kubeflow/common](https://github.com/kubeflow/common) spec
* Allow to specify the name of the gang scheduler and priority for pod group
* Added error log when pod spec does not have any containers
* Switched to use distroless images
* Refactored the kubectl-delivery to improve the launcher performance
* Added Prometheus metrics for job monitoring
* Added experimental version of v1 MPIJob controller and APIs
* Support Volcano as a scheduler
* Switched to use pods for launcher job and statefulset workers
* Switched to use klog for logging
* More consistent labels with other Kubeflow operators

### Fixes

* Fixed nil pointer exceptions that could accidentally restart the pod
* Updated status to running only when launcher is active and all workers are ready
* Fixed the incorrect namespace for initializing informers and endpoints of leader election
* Fixed issue in v1 controller's CRD existence check

### Documentation

* Added the [list of adopters](https://github.com/kubeflow/mpi-operator/blob/master/ADOPTERS.md) 
* Added [roadmap document](https://github.com/kubeflow/mpi-operator/blob/master/ROADMAP.md)
* Revamped [contributing guidelines](https://github.com/kubeflow/mpi-operator/blob/master/CONTRIBUTING.md)
* Added [MPIJob API reference page](https://www.kubeflow.org/docs/reference/mpijob/) on Kubeflow website
* Added [a blog post](https://medium.com/kubeflow/introduction-to-kubeflow-mpi-operator-and-industry-adoption-296d5f2e6edc) for an introduction to MPI Operator and its industry adoption
* Added a CPU-only example
* Added licenses used by the dependencies

## Release v0.2.2

* Added default resource requirements for init container
* Merged multiple deployment configuration files into a single YAML file
* Switched to use `JobStatus` from kubeflow/common
* Launcher and workers are now created together

## Release v0.2.1

* Switch Docker files and examples to use `v1alpha2` MPI Operator.

## Release v0.2.0

### API Changes

* Add v1alpha2 version of the MPI Operator with more consistent API spec with other Kubeflow operators
* Support `ActiveDeadlineSeconds` in `MPIJobSpec`
* Support custom resource types other than GPUs
* Remove `launcherOnMaster` field

### Enhancements

* Support gang scheduling
* Add `StartTime` and `CompletionTime` in job status
* Add leader election
* Switch to use pod group for gang scheduling
* Add example on Apache MXNet using v1alpha1 version of the MPI Operator

## Release v0.1.0

Initial release of the MPI Operator.
