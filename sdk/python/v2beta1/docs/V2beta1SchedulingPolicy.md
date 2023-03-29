# V2beta1SchedulingPolicy

SchedulingPolicy encapsulates various scheduling policies of the distributed training job, for example `minAvailable` for gang-scheduling. Now, it supports only for volcano and scheduler-plugins.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**min_available** | **int** | MinAvailable defines the minimal number of member to run the PodGroup. If the gang-scheduling isn&#39;t empty, input is passed to &#x60;.spec.minMember&#x60; in PodGroup. Note that, when using this field, you need to make sure the application supports resizing (e.g., Elastic Horovod).  If not set, it defaults to the number of workers. | [optional] 
**min_resources** | [**dict(str, ResourceQuantity)**](ResourceQuantity.md) | MinResources defines the minimal resources of members to run the PodGroup. If the gang-scheduling isn&#39;t empty, input is passed to &#x60;.spec.minResources&#x60; in PodGroup for scheduler-plugins. | [optional] 
**priority_class** | **str** | PriorityClass defines the PodGroup&#39;s PriorityClass. If the gang-scheduling is set to the volcano, input is passed to &#x60;.spec.priorityClassName&#x60; in PodGroup for volcano, and if it is set to the scheduler-plugins, input isn&#39;t passed to PodGroup for scheduler-plugins. | [optional] 
**queue** | **str** | Queue defines the queue name to allocate resource for PodGroup. If the gang-scheduling is set to the volcano, input is passed to &#x60;.spec.queue&#x60; in PodGroup for the volcano, and if it is set to the scheduler-plugins, input isn&#39;t passed to PodGroup. | [optional] 
**schedule_timeout_seconds** | **int** | SchedulerTimeoutSeconds defines the maximal time of members to wait before run the PodGroup. If the gang-scheduling is set to the scheduler-plugins, input is passed to &#x60;.spec.scheduleTimeoutSeconds&#x60; in PodGroup for the scheduler-plugins, and if it is set to the volcano, input isn&#39;t passed to PodGroup. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


