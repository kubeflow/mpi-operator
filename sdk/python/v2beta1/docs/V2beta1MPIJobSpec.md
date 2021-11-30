# V2beta1MPIJobSpec


## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mpi_implementation** | **str** | MPIImplementation is the MPI implementation. Options are \&quot;OpenMPI\&quot; (default) and \&quot;Intel\&quot;. | [optional] 
**mpi_replica_specs** | [**dict(str, V1ReplicaSpec)**](V1ReplicaSpec.md) | MPIReplicaSpecs contains maps from &#x60;MPIReplicaType&#x60; to &#x60;ReplicaSpec&#x60; that specify the MPI replicas to run. | 
**run_policy** | [**V1RunPolicy**](V1RunPolicy.md) |  | [optional] 
**slots_per_worker** | **int** | Specifies the number of slots per worker used in hostfile. Defaults to 1. | [optional] 
**ssh_auth_mount_path** | **str** | SSHAuthMountPath is the directory where SSH keys are mounted. Defaults to \&quot;/root/.ssh\&quot;. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


