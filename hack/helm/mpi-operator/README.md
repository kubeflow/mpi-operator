# Kubeflow MPI operator

Provides installation of the MPI job CRDs, RBAC and the MPI-operator service

## Chart Details

This chart will do one or more of the following:

* Install Kubeflow MPI job CRDs
* Install Kubeflow MPI job ClusterRole (clusterrole=true)
* Install The MPI deployment (operator pod) and the needed rbac (namespaced)

## Installing the Chart

To install the chart with the release name `my-release`:

```bash
$ helm install --name my-release <project-dir>/hack/helm/mpi-operator
```

## Configuration

Configurable values are documented in the `values.yaml`.

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
$ helm install --name my-release -f values.yaml <project-dir>/hack/helm/mpi-operator
```

> **Tip**: You can use the default [values.yaml](values.yaml)
