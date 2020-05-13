## <a name="sourceprocessor"></a>Source Processor
 
The `sourceprocessor` adds _source and other tags related to Sumo Logic metadata taxonomy

It leverages data tagged by `k8sprocessor` and must be after it in the processing chain. It has
certain expectations on the label names used by `k8sprocessor`

### Config
	
- `collector` (default = ""): name of the collector, put in `collector` tag
- `source_name` (default = "%{namespace}.%{pod}.%{container}"): `_sourceName` template
- `source_category` (default = "%{namespace}/%{pod_name}"): `_sourceCategory` template
- `source_category_prefix` (default = "kubernetes/"): prefix added before each `_sourceCategory` value
- `source_category_replace_dash` (default = "/"): character which all dashes (`-`) are being replaced to
- `exclude_namespace_regex` (default = empty): all data with matching namespace will be excluded
- `exclude_pod_regex` (default = empty): all data with matching pod will be excluded
- `exclude_container_regex` (default = empty): all data with matching container name will be excluded
- `exclude_host_regex` (default = empty): all data with matching `_sourceHost` will be excluded
