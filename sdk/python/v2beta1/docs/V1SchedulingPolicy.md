# V1SchedulingPolicy

SchedulingPolicy encapsulates various scheduling policies of the distributed training job, for example `minAvailable` for gang-scheduling.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**min_available** | **int** |  | [optional] 
**min_resources** | [**dict(str, ResourceQuantity)**](ResourceQuantity.md) |  | [optional] 
**priority_class** | **str** |  | [optional] 
**queue** | **str** |  | [optional] 
**schedule_timeout_seconds** | **int** |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


