# MPI Operator

[![Build Status](https://travis-ci.org/kubeflow/mpi-operator.svg?branch=master)](https://travis-ci.org/kubeflow/mpi-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubeflow/mpi-operator)](https://goreportcard.com/report/github.com/kubeflow/mpi-operator)


The MPI Operator makes it easy to run allreduce-style distributed training.

## Installation

If you haven’t already done so please follow the [Getting Started Guide](https://www.kubeflow.org/docs/started/getting-started/) to deploy Kubeflow.

An alpha version of MPI support was introduced with Kubeflow 0.2.0. You must be using a version of Kubeflow newer than 0.2.0.

You can check whether the MPI Job custom resource is installed via:

```
kubectl get crd
```

The output should include `mpijobs.kubeflow.org` like the following:

```
NAME                                       AGE
...
mpijobs.kubeflow.org                       4d
...
```

If it is not included you can add it as follows:

```
cd ${KSONNET_APP}
ks pkg install kubeflow/mpi-job
ks generate mpi-operator mpi-operator
ks apply ${ENVIRONMENT} -c mpi-operator
```

Alternatively, you can deploy the operator with default settings without using ksonnet by running the following from the repo:

```shell
kubectl create -f deploy/crd/crd-v1alpha2.yaml
kubectl create -f deploy/
```

## Creating an MPI Job

You can create an MPI job by defining an `MPIJob` config file. See [Tensorflow benchmark example](https://github.com/kubeflow/mpi-operator/blob/master/examples/v1alpha2/tensorflow-benchmarks.yaml) config file for launching a multi-node TensorFlow benchmark training job. You may change the config file based on your requirements.

```
cat examples/v1alpha2/tensorflow-benchmarks.yaml
```
Deploy the `MPIJob` resource to start training:

```
kubectl create -f examples/v1alpha2/tensorflow-benchmarks.yaml
```

## Monitoring an MPI Job

Once the `MPIJob` resource is created, you should now be able to see the created pods matching the specified number of GPUs. You can also monitor the job status from the status section. Here is sample output when the job is successfully completed.

```
kubectl get -o yaml mpijobs tensorflow-benchmarks
```

```
apiVersion: kubeflow.org/v1alpha2
kind: MPIJob
metadata:
  creationTimestamp: "2019-07-09T22:15:51Z"
  generation: 1
  name: tensorflow-benchmarks
  namespace: default
  resourceVersion: "5645868"
  selfLink: /apis/kubeflow.org/v1alpha2/namespaces/default/mpijobs/tensorflow-benchmarks
  uid: 1c5b470f-a297-11e9-964d-88d7f67c6e6d
spec:
  cleanPodPolicy: Running
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          containers:
          - command:
            - mpirun
            - --allow-run-as-root
            - -np
            - "2"
            - -bind-to
            - none
            - -map-by
            - slot
            - -x
            - NCCL_DEBUG=INFO
            - -x
            - LD_LIBRARY_PATH
            - -x
            - PATH
            - -mca
            - pml
            - ob1
            - -mca
            - btl
            - ^openib
            - python
            - scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py
            - --model=resnet101
            - --batch_size=64
            - --variable_update=horovod
            image: mpioperator/tensorflow-benchmarks:latest
            name: tensorflow-benchmarks
    Worker:
      replicas: 1
      template:
        spec:
          containers:
          - image: mpioperator/tensorflow-benchmarks:latest
            name: tensorflow-benchmarks
            resources:
              limits:
                nvidia.com/gpu: 2
  slotsPerWorker: 2
status:
  completionTime: "2019-07-09T22:17:06Z"
  conditions:
  - lastTransitionTime: "2019-07-09T22:15:51Z"
    lastUpdateTime: "2019-07-09T22:15:51Z"
    message: MPIJob default/tensorflow-benchmarks is created.
    reason: MPIJobCreated
    status: "True"
    type: Created
  - lastTransitionTime: "2019-07-09T22:15:54Z"
    lastUpdateTime: "2019-07-09T22:15:54Z"
    message: MPIJob default/tensorflow-benchmarks is running.
    reason: MPIJobRunning
    status: "False"
    type: Running
  - lastTransitionTime: "2019-07-09T22:17:06Z"
    lastUpdateTime: "2019-07-09T22:17:06Z"
    message: MPIJob default/tensorflow-benchmarks successfully completed.
    reason: MPIJobSucceeded
    status: "True"
    type: Succeeded
  replicaStatuses:
    Launcher:
      succeeded: 1
    Worker: {}
  startTime: "2019-07-09T22:15:51Z"
```


Training should run for 100 steps and takes a few minutes on a GPU cluster. You can inspect the logs to see the training progress. When the job starts, access the logs from the `launcher` pod:

```
PODNAME=$(kubectl get pods -l mpi_job_name=tensorflow-benchmarks,mpi_role_type=launcher -o name)
kubectl logs -f ${PODNAME}
```

```
TensorFlow:  1.14
Model:       resnet101
Dataset:     imagenet (synthetic)
Mode:        training
SingleSess:  False
Batch size:  128 global
             64 per device
Num batches: 100
Num epochs:  0.01
Devices:     ['horovod/gpu:0', 'horovod/gpu:1']
NUMA bind:   False
Data format: NCHW
Optimizer:   sgd
Variables:   horovod

...

40	images/sec: 154.4 +/- 0.7 (jitter = 4.0)	8.280
40	images/sec: 154.4 +/- 0.7 (jitter = 4.1)	8.482
50	images/sec: 154.8 +/- 0.6 (jitter = 4.0)	8.397
50	images/sec: 154.8 +/- 0.6 (jitter = 4.2)	8.450
60	images/sec: 154.5 +/- 0.5 (jitter = 4.1)	8.321
60	images/sec: 154.5 +/- 0.5 (jitter = 4.4)	8.349
70	images/sec: 154.5 +/- 0.5 (jitter = 4.0)	8.433
70	images/sec: 154.5 +/- 0.5 (jitter = 4.4)	8.430
80	images/sec: 154.8 +/- 0.4 (jitter = 3.6)	8.199
80	images/sec: 154.8 +/- 0.4 (jitter = 3.8)	8.404
90	images/sec: 154.6 +/- 0.4 (jitter = 3.7)	8.418
90	images/sec: 154.6 +/- 0.4 (jitter = 3.6)	8.459
100	images/sec: 154.2 +/- 0.4 (jitter = 4.0)	8.372
100	images/sec: 154.2 +/- 0.4 (jitter = 4.0)	8.542
----------------------------------------------------------------
total images/sec: 308.27
```

# Docker Images

Docker images are built and pushed automatically to [mpioperator on Dockerhub](https://hub.docker.com/u/mpioperator). You can use the following Dockerfiles to build the images yourself:

* [mpi-operator](https://github.com/kubeflow/mpi-operator/blob/master/Dockerfile)
* [kubectl-delivery](https://github.com/kubeflow/mpi-operator/blob/master/cmd/kubectl-delivery/Dockerfile)
