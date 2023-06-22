# Pure MPI example

This example shows to run a pure MPI application.

The program prints some basic information about the workers.
Then, it calculates an approximate value for pi.

## How to build Image

For OpenMPI:

```bash
docker build -t mpi-pi .
```

For Intel MPI:

```bash
docker build -t mpi-pi . -f intel.Dockerfile
```

For MPICH:

```bash
docker build -t mpi-pi . -f mpich.Dockerfile
```

## Create MPIJob

Modify `pi.yaml` (for OpenMPI), `pi-intel.yaml` (for Intel MPI) or `pi-mpich.yaml` (for MPICH) to set up the
image name from your own registry.

Then, run:

```
kubectl create -f pi.yaml
```

The YAML shows how to run the binaries as a non-root user.