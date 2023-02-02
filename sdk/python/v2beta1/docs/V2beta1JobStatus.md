# V2beta1JobStatus

JobStatus represents the current observed state of the training Job.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**completion_time** | [**V1Time**](V1Time.md) |  | [optional] 
**conditions** | [**list[V2beta1JobCondition]**](V2beta1JobCondition.md) | conditions is a list of current observed job conditions. | [optional] 
**last_reconcile_time** | [**V1Time**](V1Time.md) |  | [optional] 
**replica_statuses** | [**dict(str, V2beta1ReplicaStatus)**](V2beta1ReplicaStatus.md) | replicaStatuses is map of ReplicaType and ReplicaStatus, specifies the status of each replica. | [optional] 
**start_time** | [**V1Time**](V1Time.md) |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


