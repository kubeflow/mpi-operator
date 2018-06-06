# MPI Operator

The MPI Operator makes it easy to run allreduce-style distributed training.

## Deploy

```shell
kubectl create -f deploy/
```

## Test

Launch a multi-node tensorflow benchmark training job:
```shell
kubectl create -f examples/tensorflow-benchmarks.yaml
```

Once everything starts, the logs are available in the `launcher` pod.
