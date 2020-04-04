# Horovod CPU-Only Case

This example shows how to run a cpu-only mpijob.

## How to Build Image

Please download Horovod cpu only dockerfile and then build the image as follows:

```bash
curl -O https://raw.githubusercontent.com/horovod/horovod/master/Dockerfile.cpu
# change the default tensorflow version from 2.1.0 to 1.14.0,
# otherwise, the example would't run successfully
sed -i 's/TENSORFLOW_VERSION=2.1.0/TENSORFLOW_VERSION=1.14.0/' Dockerfile.cpu
docker build -t horovod:latest -f Dockerfile.cpu .
```

## Create Mpijob

```bash
kubectl create -f ./tensorflow-mnist.yaml
```
