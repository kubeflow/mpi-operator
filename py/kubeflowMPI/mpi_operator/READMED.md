# Instructions for setup

```bash
export PYTHONPATH=${KUBEFLOW_TESTING}/py:${KUBEFLOW_MPI}/py
```

```bash
export KUBEFLOW_TESTING=/Users/niklashansson/Documents/OpenSource/testing
export KUBEFLOW_MPI=/Users/niklashansson/Documents/OpenSource/mpi-operator
```

```bash
python kubeflowMPI/mpi_operator/e2e_tool.py apply --name=${USER}-kfctl-test-$(date +%Y%m%d-%H%M%S)   --namespace=kubeflow-test-infra   --open-in-chrome=true --test_target_name e2etest_mpi_operator
```

Set the needed KUBECONFIG enviroment variable. 
```bash
export KUBECONFIG= $HOME/.kube/config
```