# Pure MPI example

This example shows to run a pure MPI application.

The program prints some basic information about the workers.
Then, it calculates an approximate value for pi.

## How to build Image

```bash
docker build -t mpi-pi .
```

## Create MPIJob

Modify `pi.yaml` to set up the image name from your own registry.

Then, run:

```
kubectl create -f pi.yaml
```

The YAML shows how to run the binaries as a non-root user.