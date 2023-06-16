# A scalable and robust operator

Authors: @alculquicondor, @ahg-g

- [Motivation](#motivation)
- [Goals](#goals)
- [Background](#background)
- [Design](#design)
- [Alternatives Considered](#alternatives-considered)
- [Appendix](#appendix-prototype-objects)

## Motivation

A scalable MPI setup on Kubernetes is important for:
- Running jobs from multiple users in a single cluster.
- High-performance computing jobs that can require a big number of workers.

A robust MPI setup should be tolerant to failures, implementing retries while
keeping track of failures.

## Goals

- Allow driver-to-worker control to scale by removing kube-apiserver from
  the communication channel.
- Reduce the complexity of the controller by relaying on Kubernetes workload
  APIs for Pod creation and management.
  
## Background

The original design of the operator can be found as a
[proposal to the kubeflow community](https://github.com/kubeflow/community/blob/master/proposals/mpi-operator-proposal.md).
The latest release includes a v1alpha2 API and controller.
A v1 is under development. Something to highlight in this new version is the
replacement of the Job and StatefulSet by plain Pods, with the intent of
[tracking running Pods](https://github.com/kubeflow/mpi-operator/issues/201#issuecomment-827837831).

An MPIJob CRD describes the Job. Important fields include:
- The workers template
- The number of workers
- The launcher template, which should have a `mpirun` command.

The images are expected to have the MPI implementation binaries (such as
OpenMPI, Intel MPI or MPICH) the user’s MPI executable.

A controller processes the MPIJob, starting a Job with the following steps:
1. Creates ConfigMap, which contains:
  - A script `kubexec.sh` that wraps `kubectl exec` and is used in replacement
    of `ssh`. This script, before executing the command provided by `mpirun`,
    transfers a file containing a mapping of pod names to IPs and appends it to
    the worker’s `/etc/hosts`.
    
    Note: The v1 controller no longer copies the pod-to-IP mapping file. The
    OpenMPI implementation does the [routing](https://www.open-mpi.org/faq/?category=tcp#tcp-routability-1.3).
  - The `hostfile` for `mpirun`, listing the worker pod names and number of
    slots (which could be the number of CPUs or GPUs). This list is built
    programmatically.
2. Creates a ServiceAccount+Role+RoleBinding for the launcher, which allow it
  to:
  - get/list/watch on Pods
  - do pods/exec
  This allows the launcher Pod to obtain details of the worker Pods and start
  the process managers on them.
3. If configured, it creates a Volcano PodGroup
4. Creates a StatefulSet for workers (plain pods in v1). The Pod template
  includes:
  - mounts for the ConfigMap.
  - `sleep` as command.
5. Creates launcher Job (plain pod in v1). The Pod template includes:
  - An init container, `kubectl-delivery`, described below.
  - Environment variables for:
    - replacing `ssh` for `kubexec.sh`
    - the `hostfile` location
  - Volumes for:
    - The ConfigMap
    - Sharing files from `kubectl-delivery`

The launcher Job, as previously mentioned, contains an init container:
`kubectl-delivery`. This is a Kubernetes controller that watches pods. It does
the following:

1. Copy kubectl from the image into the volume shared with the main container.
2. Wait for all Pods in the hostfile to be running
3. Generates a file mapping pod name to IP, in `/etc/hosts` format.

To update the status of an MPIJob, the controller uses the status of the
launcher Job. That is, when the launcher Job fails or succeeds, it’s status is
copied to the MPIJob. In v1, it bases the status on the termination condition of
the Pod.

### Analysis

The above architecture for MPI Jobs puts a lot of pressure in the
`kube-apiserver`. The load increases with the number of workers in a job
and with the number of jobs in a cluster.

The reasons for this are:
- Due to the use of `kubectl exec`, every worker spawn goes through
  kube-apiserver. `mpirun` starts a daemon in each worker
  (like [`orted`](https://www.open-mpi.org/doc/v3.0/man1/orted.1.php)).
  This process handles the worker-to-worker communication, which happens
  without the intervention of kube-apiserver. However, the `exec` connection
  stays up for control during the entirety of the job.
- The `kubectl-delivery` controller does a full cache sync to be able to watch
  Pods. This startup penalty increases with the number of pods in the cluster
  and has to be paid for every job. The API calls also cause additional
  stress on the apiserver.
- The launcher role has to include a list of all the pods in the job.
  Potentially, the object might not be able to accommodate jobs with immense
  number of workers.
  
Another problem is that the v1 controller doesn’t implement launcher pod
retries, although there are plans to. So the MPIJob behaves like a plain Pod in
this version.

## Design

In order to address the problems of the existing architecture, we propose the
following changes:

- **The use of `ssh` instead of `kubectl exec`.**
  
  This would avoid any pod-to-pod communication happening through apiserver and
  doesn’t require giving execution permissions at the namespace level.
  This can be achieved like this:
  - The controller generates a single key and share it with the launcher and
    worker pods through a Secret.
  - The launcher and workers mount the Secret and set appropriate file
    permissions.
  - The workers run an SSH server instead of `sleep`.
  - When encrypted communication is not a requirement, users have the choice to
    use `rsh` for faster communication.

- **The use of stable hostnames and a headless Service for the workers**
  - This removes the need to query Pod IPs, as the Pods can discover each other
    through DNS resolution. The hostfile can be generated statically by the
    controller using the stable hostnames.
  - Starting with k8s 1.22, we can use
    [Indexed Jobs with stable hostnames](https://git.k8s.io/enhancements/keps/sig-apps/2214-indexed-job)
    to delegate the pod management to Kubernetes. Additionally, this will give
    us robust failure tracking so that we can give users control over retry
    limits.
    In the meantime, we can continue using plain Pods.
    - Caveat 1: The Job controller doesn’t report the number of running Pods.
      Instead, it reports active Pods, which include running and pending
      (scheduling or starting). But Kubernetes SIG Apps is
      [open to add a status field for running Pods](https://kubernetes.slack.com/archives/C18NZM5K9/p1619549430067400).
    - Caveat 2: Horovod supports elastic workers, but the Kubernetes Job
      doesn’t support changes to the completions field. This can be supported
      starting from 1.23. In the meantime, we can replicate the behavior by
      creating a new Job and doing Pod adoption.
  - For Intel MPI and MPICH, we also need a headless Service to front the launcher,
    because workers communicate back to the launcher using its hostname.
- **Revert the use of the Job API for the launcher.**
  - The Job controller handles retries when the launcher or any of the workers fail.
  - Caveat 1 also applies: The Job controller doesn’t report if the Pod is running.
    We can continue watching Pods in the meantime.
- With the above changes, **the following objects can be removed**:
  - The ServiceAccount+Role+RoleBinding for the launcher.
  - The `kubectl-delivery` init container in the launcher, as there is no need
    to obtain IPs, speeding up startup time.
    
## Alternatives Considered

TBD from discussions

## Appendix: Prototype objects

It uses a StatefulSet in place of Indexed Jobs, as they are still an alpha
feature in Kubernetes.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mpi-config
data:
  hostfile: |
    mpi-workers-0.mpi-workers slots=3
    mpi-workers-1.mpi-workers slots=3
    mpi-workers-2.mpi-workers slots=3
```

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/ssh-auth
data:
  ssh-privatekey: PRIVATE_KEY
  ssh-publickey: PUBLIC_KEY
```

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: mpi-launcher
spec:
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: driver
        image: '<USER_IMAGE>'
        args:
        - 'mpirun'
        - '-np'
        - '9'
        - '<USER_EXECUTABLE>'
        env:
        - name: 'OMPI_MCA_orte_keep_fqdn_hostnames'
          value: 'true'
        - name: 'OMPI_MCA_orte_default_hostfile'
          value: '/home/mpiuser/config/hostfile'
        volumeMounts:
        - name: ssh-auth
          mountPath: /mnt/ssh
          readOnly: true
        - name: mpi-config
          mountPath: /home/mpiuser/config
          readOnly: true
      volumes:
      - name: mpi-config
        configMap:
          name: mpi-config
      - name: ssh-auth
        secret:
          secretName: ssh-auth
          items:
          - key: ssh-privatekey
            path: id_rsa
          - key: ssh-publickey
            path: id_rsa.pub
          - key: ssh-publickey
            path: authorized_keys
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mpi-workers
spec:
  clusterIP: None
  selector:
    app: mpi-workers
```

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mpi-workers
spec:
  selector:
    matchLabels:
      app: mpi-workers
  serviceName: mpi-workers
  replicas: 3
  podManagementPolicy: Parallel
  template:
    metadata:
      labels:
        app: mpi-workers
    spec:
      containers:
      - name: worker
        image: '<USER_IMAGE>'
        volumeMounts:
        - name: ssh-auth
          mountPath: /mnt/ssh
          readOnly: true
        - name: mpi-config
          mountPath: /home/mpiuser/config
          readOnly: true
      volumes:
      - name: mpi-config
        configMap:
          name: mpi-config
      - name: ssh-auth
        secret:
          secretName: ssh-auth
          items:
          - key: ssh-privatekey
            path: id_rsa
          - key: ssh-publickey
            path: id_rsa.pub
          - key: ssh-publickey
            path: authorized_keys
```