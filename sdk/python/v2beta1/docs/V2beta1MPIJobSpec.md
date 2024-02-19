# V2beta1MPIJobSpec


## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**launcher_creation_policy** | **str** | launcherCreationPolicy if WaitForWorkersReady, the launcher is created only after all workers are in Ready state. Defaults to AtStartup. | [optional] 
**mpi_implementation** | **str** | MPIImplementation is the MPI implementation. Options are \&quot;OpenMPI\&quot; (default), \&quot;Intel\&quot; and \&quot;MPICH\&quot;. | [optional] 
**mpi_replica_specs** | [**dict(str, V2beta1ReplicaSpec)**](V2beta1ReplicaSpec.md) | MPIReplicaSpecs contains maps from &#x60;MPIReplicaType&#x60; to &#x60;ReplicaSpec&#x60; that specify the MPI replicas to run. | 
**run_launcher_as_worker** | **bool** | RunLauncherAsWorker indicates whether to run worker process in launcher Defaults to false. | [optional] 
**run_policy** | [**V2beta1RunPolicy**](V2beta1RunPolicy.md) |  | [optional] 
**slots_per_worker** | **int** | Specifies the number of slots per worker used in hostfile. Defaults to 1. | [optional] 
**ssh_auth_mount_path** | **str** | SSHAuthMountPath is the directory where SSH keys are mounted. Defaults to \&quot;/root/.ssh\&quot;. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


