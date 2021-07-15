# Horovod CPU-Only Case

This example shows how to run a cpu-only mpijob.

## How to Build Image

This example dockerfile is based on Horovod cpu only [dockerfile](https://raw.githubusercontent.com/horovod/horovod/master/Dockerfile.cpu), please build the image as follows:

```bash
docker build -t horovod:latest .
```

## Create Mpijob

The example mpijob is to run the horovod cpu-only example [tensorflow_mnist.py](https://raw.githubusercontent.com/horovod/horovod/master/examples/tensorflow_mnist.py).

```bash
kubectl create -f ./tensorflow-mnist.yaml
```
