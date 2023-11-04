# Horovod CPU-Only Case

This example shows how to run a cpu-only mpijob.

## How to Build Image

This example dockerfile is based on Horovod cpu only [dockerfile](https://raw.githubusercontent.com/horovod/horovod/master/docker/horovod-cpu/Dockerfile), you can build the image as follows:

```bash
docker build -t horovod:latest .
```

## Create Mpijob

The example mpijob is to run the horovod cpu-only example [tensorflow_mnist.py](https://raw.githubusercontent.com/horovod/horovod/master/examples/tensorflow/tensorflow_mnist.py).

```bash
kubectl create -f ./tensorflow-mnist.yaml
```
## v1 MPI job

For old API kubeflow.org/v1 deploy manifest see [tensorflow-mnist-elastic.yaml](https://raw.githubusercontent.com/kubeflow/mpi-operator/v0.3.0/examples/horovod/tensorflow-mnist-elastic.yaml).