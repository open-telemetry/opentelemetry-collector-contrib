## <a name="k8sprocessor"></a>Kubernetes Processor
 
The `k8sprocessor` allow automatic tagging of spans with k8s metadata.

It automatically discovers k8s resources (pods), extracts metadata from them and adds theextracted 
metadata to the relevant spans. The processor use the kubernetes API to discover all pods running 
in a cluster, keeps a record of their IP addresses and interesting metadata. Upon receiving spans,
the processor tries to identify the source IP address of the service that sent the spans and matches
it with the in memory data. If a match is found, the cached metadata is added to the spans as attributes.

### Config

There are several top level sections of the processor config:

- `passthrough` (default = false): when set to true, only annotates resources with the pod IP and
does not try to extract any other metadata. It does not need access to the K8S cluster API. 
Agent/Collector must receive spans directly from services to be able to correctly detect the pod IPs.
- `owner_lookup_enabled` (default = false): when set to true, fields such as `daemonSetName`, 
`replicaSetName`, `service`, etc. can be extracted, though it requires fetching additional data to traverse 
the `owner` relationship.  See the [list of fields](#k8sprocessor-extract) for more information over 
which tags require the flag to be enabled. 
- `extract`: the section (see [below](#k8sprocessor-extract)) allows specifying extraction rules
- `filter`: the section (see [below](#k8sprocessor-filter)) allows specifying filters when matching pods

#### <a name="k8sprocessor-extract"></a>Extract section

Allows specifying extraction rules to extract data from k8s pod specs.

- `metadata` (default = empty): specifies a list of strings that denote extracted fields. Following fields
can be extracted:
    - `containerId`
    - `containerName`
    - `containerImage`
    - `clusterName`
    - `daemonSetName` _(`owner_lookup_enabled` must be set to `true`)_
    - `deploymentName`
    - `hostName`
    - `namespace`
    - `nodeName`
    - `podId`
    - `podName`
    - `replicaSetName` _(`owner_lookup_enabled` must be set to `true`)_
    - `serviceName` _(`owner_lookup_enabled` must be set to `true`)_ - in case more than one service is assigned 
    to the pod, they are comma-separated
    - `startTime`
    - `statefulSetName` _(`owner_lookup_enabled` must be set to `true`)_
      
    Also, see [example config](#k8sprocessor-example). 
- `tags`: specifies an optional map of custom tag names to be used. By default, following names are being assigned:
	- `clusterName    `: `k8s.cluster.name`
	- `containerID    `: `k8s.container.id`
	- `containerImage `: `k8s.container.image`
	- `containerName  `: `k8s.container.name`
	- `daemonSetName  `: `k8s.daemonset.name`
	- `deploymentName `: `k8s.deployment.name`
	- `hostName       `: `k8s.pod.hostname`
	- `namespaceName  `: `k8s.namespace.name`
	- `nodeName       `: `k8s.node.name`
	- `podID          `: `k8s.pod.id`
	- `podName        `: `k8s.pod.name`
	- `replicaSetName `: `k8s.replicaset.name`
	- `serviceName    `: `k8s.service.name`
	- `statefulSetName`: `k8s.statefulset.name`
	- `startTime      `: `k8s.pod.startTime`

    When custom value is specified, specified fields use provided names when being tagged, e.g.:
    ```yaml
    tags:
      containerId: my-custom-tag-for-container-id
      nodeName: node_name
    ```
 - `annotations` (default = empty): a list of rules for extraction and recording annotation data.
See [field extract config](#k8sprocessor-field-extract) for an example on how to use it.
- `labels` (default = empty): a list of rules for extraction and recording label data.
See [field extract config](#k8sprocessor-field-extract) for an example on how to use it.
- `namespace_labels` (default = empty): a list of rules for extraction and recording namespace label data.
See [field extract config](#k8sprocessor-field-extract) for an example on how to use it.

#### <a name="k8sprocessor-field-extract"></a> Field Extract Config

Allows specifying an extraction rule to extract a value from exactly one field.

The field accepts a list of maps accepting three keys: `tag-name`, `key` and `regex`

- `tag-name`: represents the name of the tag that will be added to the span.  When not specified 
a default tag name will be used of the format: `k8s.<annotation>.<annotation key>` For example, if 
`tag-name` is not specified and the key is `git_sha`, then the span name will be `k8s.annotation.deployment.git_sha`

- `key`: represents the annotation name. This must exactly match an annotation name. To capture 
all keys, `*` can be used

- `regex`: is an optional field used to extract a sub-string from a complex field value.
The supplied regular expression must contain one named parameter with the string "value"
as the name. For example, if your pod spec contains the following annotation,
`kubernetes.io/change-cause: 2019-08-28T18:34:33Z APP_NAME=my-app GIT_SHA=58a1e39 CI_BUILD=4120`
and you'd like to extract the GIT_SHA and the CI_BUILD values as tags, then you must specify 
the following two extraction rules:
  
  ```yaml
  procesors:
    k8s-tagger:
      annotations:
        - tag_name: git.sha
          key: kubernetes.io/change-cause
          regex: GIT_SHA=(?P<value>\w+)
        - tag_name: ci.build
          key: kubernetes.io/change-cause
          regex: JENKINS=(?P<value>[\w]+)
  ```

  this will add the `git.sha` and `ci.build` tags to the spans. It is also possible to generically fetch 
  all keys and fill them into a template. To substitute the original name, use `%s`. For example:

  ```yaml
  procesors:
    k8s-tagger:
      annotations:
        - tag_name: k8s.annotation/%s
          key: *
  ```
          
#### <a name="k8sprocessor-filter"></a>Filter section

FilterConfig section allows specifying filters to filter pods by labels, fields, namespaces, nodes, etc.

- `node` (default = ""): represents a k8s node or host. If specified, any pods not running on the specified 
node will be ignored by the tagger.
- `node_from_env_var` (default = ""): can be used to extract the node name from an environment variable. 
The value must be the name of the environment variable. This is useful when the node a Otel agent will 
run on cannot be predicted. In such cases, the Kubernetes downward API can be used to add the node name 
to each pod as an environment variable. K8s tagger can then read this value and filter pods by it.
For example, node name can be passed to each agent with the downward API as follows
 
    ```yaml
     env:
       - name: K8S_NODE_NAME
             valueFrom:
               fieldRef:
                 fieldPath: spec.nodeName
    ```

  Then the NodeFromEnv field can be set to `K8S_NODE_NAME` to filter all pods by the node that the agent 
  is running on. More on downward API here: 
  https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/
- `namespace` (default = ""): filters all pods by the provided namespace. All other pods are ignored.
- `fields` (default = empty): a list of maps accepting three keys: `key`, `value`, `op`. Allows to filter 
pods by generic k8s fields. Only the following operations (`op`) are supported: `equals`, `not-equals`.
For example, to match pods having `key1=value1` and `key2<>value2` condition met for fields, one can specify:
 
    ```yaml
      fields: 
       - key: key1 # `op` defaults to "equals" when not specified
         value: value1
       - key: key2 
         value: value2
         op: not-equals
    ```

- `labels` (default = empty): a list of maps accepting three keys: `key`, `value`, `op`. Allows to filter 
pods by generic k8s pod labels. Only the following operations (`op`) are supported: `equals`, `not-equals`, 
`exists`, `not-exists`. For example, to match pods where `label1` exists, one can specify

    ```yaml
      fields: 
       - key: label1
         op: exists
    ``` 

#### <a name="k8sprocessor-example"></a>Example config:

```yaml
processors:
  k8s_tagger:
    passthrough: false
    owner_lookup_enabled: true # To enable fetching additional metadata using `owner` relationship
    extract:
      metadata:
        # extract the following well-known metadata fields
        - containerId
        - containerName
        - containerImage
        - clusterName
        - daemonSetName
        - deploymentName
        - hostName
        - namespace
        - nodeName
        - podId
        - podName
        - replicaSetName
        - serviceName
        - startTime
        - statefulSetName
      tags:
        # It is possible to provide your custom key names for each of the extracted metadata fields, 
        # e.g. to store podName as "pod_name" rather than the default "k8s.pod.name", use following:
        podName: pod_name

      annotations:
        # Extract all annotations using a template
        - tag_name: k8s.annotation.%s
          key: "*"
      labels:
        - tag_name: l1 # extracts value of label with key `label1` and inserts it as a tag with key `l1`
          key: label1
        - tag_name: l2 # extracts value of label with key `label1` with regexp and inserts it as a tag with key `l2`
          key: label2
          regex: field=(?P<value>.+)

    filter:
      namespace: ns2 # only look for pods running in ns2 namespace
      node: ip-111.us-west-2.compute.internal # only look for pods running on this node/host
      node_from_env_var: K8S_NODE # only look for pods running on the node/host specified by the K8S_NODE environment variable
      labels: # only consider pods that match the following labels
       - key: key1 # match pods that have a label `key1=value1`. `op` defaults to "equals" when not specified
         value: value1
       - key: key2 # ignore pods that have a label `key2=value2`.
         value: value2
         op: not-equals
      fields: # works the same way as labels but for fields instead (like annotations)
       - key: key1
         value: value1
       - key: key2
         value: value2
         op: not-equals
```

### RBAC

TODO: mention the required RBAC rules.

### Deployment scenarios

The processor supports running both in agent and collector mode.

#### As an agent

When running as an agent, the processor detects IP addresses of pods sending spans to the agent and uses this
information to extract metadata from pods and add to spans. When running as an agent, it is important to apply
a discovery filter so that the processor only discovers pods from the same host that it is running on. Not using
such a filter can result in unnecessary resource usage especially on very large clusters. Once the fitler is applied,
each processor will only query the k8s API for pods running on it's own node.

Node filter can be applied by setting the `filter.node` config option to the name of a k8s node. While this works
as expected, it cannot be used to automatically filter pods by the same node that the processor is running on in
most cases as it is not know before hand which node a pod will be scheduled on. Luckily, kubernetes has a solution
for this called the downward API. To automatically filter pods by the node the processor is running on, you'll need
to complete the following steps:

1. Use the downward API to inject the node name as an environment variable.
Add the following snippet under the pod env section of the OpenTelemetry container.

    ```yaml
       env:
       - name: KUBE_NODE_NAME
         valueFrom:
     	  fieldRef:
 	        apiVersion: v1
 	        fieldPath: spec.nodeName
    ```

    This will inject a new environment variable to the OpenTelemetry container with the value as the
    name of the node the pod was scheduled to run on.

2. Set "filter.node_from_env_var" to the name of the environment variable holding the node name.

    ```yaml
       k8s_tagger:
         filter:
           node_from_env_var: KUBE_NODE_NAME # this should be same as the var name used in previous step
    ```

    This will restrict each OpenTelemetry agent to query pods running on the same node only dramatically reducing
    resource requirements for very large clusters.

#### As a collector

The processor can be deployed both as an agent or as a collector.

When running as a collector, the processor cannot correctly detect the IP address of the pods generating
the spans when it receives the spans from an agent instead of receiving them directly from the pods. To
workaround this issue, agents deployed with the k8s_tagger processor can be configured to detect
the IP addresses and forward them along with the span resources. Collector can then match this IP address
with k8s pods and enrich the spans with the metadata. In order to set this up, you'll need to complete the
following steps:

1. Setup agents in passthrough mode
Configure the agents' k8s_tagger processors to run in passthrough mode.

    ```yaml
       # k8s_tagger config for agent
       k8s_tagger:
         passthrough: true
    ```
    This will ensure that the agents detect the IP address as add it as an attribute to all span resources.
    Agents will not make any k8s API calls, do any discovery of pods or extract any metadata.

2. Configure the collector as usual
No special configuration changes are needed to be made on the collector. It'll automatically detect
the IP address of spans sent by the agents as well as directly by other services/pods.


### Caveats

There are some edge-cases and scenarios where k8s_tagger will not work properly.


#### Host networking mode

The processor cannot correct identify pods running in the host network mode and
enriching spans generated by such pods is not supported at the moment.

#### As a sidecar

The processor does not support detecting containers from the same pods when running
as a sidecar. While this can be done, we think it is simpler to just use the kubernetes
downward API to inject environment variables into the pods and directly use their values
as tags.
