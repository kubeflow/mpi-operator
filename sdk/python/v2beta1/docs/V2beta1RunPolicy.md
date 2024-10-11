# V2beta1RunPolicy

RunPolicy encapsulates various runtime policies of the distributed training job, for example how to clean up resources and how long the job can stay active.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active_deadline_seconds** | **int** | Specifies the duration in seconds relative to the startTime that the job may be active before the system tries to terminate it; value must be positive integer. | [optional] 
**backoff_limit** | **int** | Optional number of retries before marking this job failed. | [optional] 
**clean_pod_policy** | **str** | CleanPodPolicy defines the policy to kill pods after the job completes. Default to Running. | [optional] 
**managed_by** | **str** | ManagedBy is used to indicate the controller or entity that manages a MPIJob. The value must be either empty, &#39;kubeflow.org/mpi-operator&#39; or &#39;kueue.x-k8s.io/multikueue&#39;. The mpi-operator reconciles a MPIJob which doesn&#39;t have this field at all or the field value is the reserved string &#39;kubeflow.org/mpi-operator&#39;, but delegates reconciling the MPIJob with &#39;kueue.x-k8s.io/multikueue&#39; to the Kueue. The field is immutable. | [optional] 
**scheduling_policy** | [**V2beta1SchedulingPolicy**](V2beta1SchedulingPolicy.md) |  | [optional] 
**suspend** | **bool** | suspend specifies whether the MPIJob controller should create Pods or not. If a MPIJob is created with suspend set to true, no Pods are created by the MPIJob controller. If a MPIJob is suspended after creation (i.e. the flag goes from false to true), the MPIJob controller will delete all active Pods and PodGroups associated with this MPIJob. Also, it will suspend the Launcher Job. Users must design their workload to gracefully handle this. Suspending a Job will reset the StartTime field of the MPIJob.  Defaults to false. | [optional] 
**ttl_seconds_after_finished** | **int** | TTLSecondsAfterFinished is the TTL to clean up jobs. It may take extra ReconcilePeriod seconds for the cleanup, since reconcile gets called periodically. Default to infinite. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


