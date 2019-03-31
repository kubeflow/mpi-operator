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
kubectl create -f deploy/
```

## Creating an MPI Job

You can create an MPI job by defining an `MPIJob` config file. See [Tensorflow benchmark example](https://github.com/kubeflow/mpi-operator/blob/master/examples/tensorflow-benchmarks.yaml) config file for launching a multi-node TensorFlow benchmark training job. You may change the config file based on your requirements.

```
cat examples/tensorflow-benchmarks.yaml
```
Deploy the `MPIJob` resource to start training:

```
kubectl create -f examples/tensorflow-benchmarks.yaml
```

## Monitoring an MPI Job

Once the `MPIJob` resource is created, you should now be able to see the created pods matching the specified number of GPUs. You can also monitor the job status from the status section. Here is sample output when the job is successfully completed.

```
kubectl get -o yaml mpijobs tensorflow-benchmarks-16
```

```
apiVersion: kubeflow.org/v1alpha1
kind: MPIJob
metadata:
  clusterName: ""
  creationTimestamp: 2019-01-07T20:32:12Z
  generation: 1
  name: tensorflow-benchmarks-16
  namespace: default
  resourceVersion: "185051397"
  selfLink: /apis/kubeflow.org/v1alpha1/namespaces/default/mpijobs/tensorflow-benchmarks-16
  uid: 8dc8c044-127d-11e9-a419-02420bbe29f3
spec:
  gpus: 16
  template:
    metadata:
      creationTimestamp: null
    spec:
      containers:
      - image: mpioperator/tensorflow-benchmarks:latest
        name: tensorflow-benchmarks
        resources: {}
status:
  launcherStatus: Succeeded
```


Training should run for 100 steps and takes a few minutes on a GPU cluster. You can inspect the logs to see the training progress. When the job starts, access the logs from the `launcher` pod:

```
PODNAME=$(kubectl get pods -l mpi_job_name=tensorflow-benchmarks-16,mpi_role_type=launcher -o name)
kubectl logs -f ${PODNAME}
```

```
TensorFlow:  1.10
Model:       resnet101
Dataset:     imagenet (synthetic)
Mode:        training
SingleSess:  False
Batch size:  128 global
             64 per device
Num batches: 100
Num epochs:  0.01
Devices:     ['horovod/gpu:0', 'horovod/gpu:1']
Data format: NCHW
Optimizer:   sgd
Variables:   horovod

...

40	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.146
40	images/sec: 132.1 +/- 0.0 (jitter = 0.1)	9.182
50	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.071
50	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.210
60	images/sec: 132.2 +/- 0.0 (jitter = 0.2)	9.180
60	images/sec: 132.2 +/- 0.0 (jitter = 0.2)	9.055
70	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.005
70	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.096
80	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.231
80	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.197
90	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.201
90	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.089
100	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.183
----------------------------------------------------------------
total images/sec: 264.26
----------------------------------------------------------------
100	images/sec: 132.1 +/- 0.0 (jitter = 0.2)	9.044
----------------------------------------------------------------
total images/sec: 264.26
----------------------------------------------------------------
```

# Docker Images

Docker images are built and pushed automatically to [mpioperator on Dockerhub](https://hub.docker.com/u/mpioperator). You can use the following Dockerfiles to build the images yourself:

* [mpi-operator](https://github.com/kubeflow/mpi-operator/blob/master/Dockerfile)
* [kubectl-delivery](https://github.com/kubeflow/mpi-operator/blob/master/cmd/kubectl-delivery/Dockerfile)
