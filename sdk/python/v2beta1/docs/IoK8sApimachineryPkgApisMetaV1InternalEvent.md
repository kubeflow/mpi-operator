# IoK8sApimachineryPkgApisMetaV1InternalEvent

InternalEvent makes watch.Event versioned

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**object** | **object** | Object is:  * If Type is Added or Modified: the new state of the object.  * If Type is Deleted: the state of the object immediately before deletion.  * If Type is Bookmark: the object (instance of a type being watched) where    only ResourceVersion field is set. On successful restart of watch from a    bookmark resourceVersion, client is guaranteed to not get repeat event    nor miss any events.  * If Type is Error: *api.Status is recommended; other types may make sense    depending on context. | 
**type** | **str** |  | [default to '']

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


