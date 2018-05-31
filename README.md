# MPI Operator

The MPI Operator makes it easy to run allreduce-style distributed training.

## Build

Check out the code:
```shell
mkdir -p ${GOPATH}/src/github.com/kubeflow
cd ${GOPATH}/src/github.com/kubeflow
git clone https://github.com/kubeflow/mpi-operator.git
cd mpi-operator
```

Build and push the `mpi-operator` Docker image:
```shell
docker built -t rongou/mpi-operator:0.1.0 -f cmd/mpi-operator/Dockerfile .
docker push rongou/mpi-operator:0.1.0
```

Build and push the `kubectl-delivery` Docker image:
```shell
docker build -t rongou/kubectl-delivery:0.1.0 -f cmd/kubectl-delivery/Dockerfile .
docker push rongou/mpi-operator:0.1.0
```

## Deploy

```shell
kubectl create -f deploy/
```

## Test

Build and push the `horovod` Docker image (this takes a while):
```shell
docker build -t rongou/horovod https://github.com/uber/horovod.git
docker push rongou/horovod
```

Build and push the `tensorflow_benchmarks` Docker image:
```shell
docker build -t rongou/tensorflow_benchmarks examples/tensorflow-benchmarks
docker push rongou/tensorflow_benchmarks
```

Launch a multi-node tensorflow benchmark training job:
```shell
kubectl create -f examples/tensorflow-benchmarks.yaml
```

Once everything starts, the logs are available in the `launcher` pod.
