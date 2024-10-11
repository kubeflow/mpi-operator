# V1TableRow

TableRow is an individual row in a table.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**cells** | **list[object]** | cells will be as wide as the column definitions array and may contain strings, numbers (float64 or int64), booleans, simple maps, lists, or null. See the type field of the column definition for a more detailed description. | 
**conditions** | [**list[V1TableRowCondition]**](V1TableRowCondition.md) | conditions describe additional status of a row that are relevant for a human user. These conditions apply to the row, not to the object, and will be specific to table output. The only defined condition type is &#39;Completed&#39;, for a row that indicates a resource that has run to completion and can be given less visual priority. | [optional] 
**object** | **object** | RawExtension is used to hold extensions in external versions.  To use this, make a field which has RawExtension as its type in your external, versioned struct, and Object in your internal struct. You also need to register your various plugin types.  // Internal package:   type MyAPIObject struct {   runtime.TypeMeta &#x60;json:\&quot;,inline\&quot;&#x60;   MyPlugin runtime.Object &#x60;json:\&quot;myPlugin\&quot;&#x60;  }   type PluginA struct {   AOption string &#x60;json:\&quot;aOption\&quot;&#x60;  }  // External package:   type MyAPIObject struct {   runtime.TypeMeta &#x60;json:\&quot;,inline\&quot;&#x60;   MyPlugin runtime.RawExtension &#x60;json:\&quot;myPlugin\&quot;&#x60;  }   type PluginA struct {   AOption string &#x60;json:\&quot;aOption\&quot;&#x60;  }  // On the wire, the JSON will look something like this:   {   \&quot;kind\&quot;:\&quot;MyAPIObject\&quot;,   \&quot;apiVersion\&quot;:\&quot;v1\&quot;,   \&quot;myPlugin\&quot;: {    \&quot;kind\&quot;:\&quot;PluginA\&quot;,    \&quot;aOption\&quot;:\&quot;foo\&quot;,   },  }  So what happens? Decode first uses json or yaml to unmarshal the serialized data into your external MyAPIObject. That causes the raw JSON to be stored, but not unpacked. The next step is to copy (using pkg/conversion) into the internal struct. The runtime package&#39;s DefaultScheme has conversion functions installed which will unpack the JSON stored in RawExtension, turning it into the correct object type, and storing it in the Object. (TODO: In the case where the object is of an unknown type, a runtime.Unknown object will be created and stored.) | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


