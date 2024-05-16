## `container` operator

The `container` operator parses logs in `docker`, `cri-o` and `containerd` formats.

### Configuration Fields

| Field                        | Default          | Description                                                                                                                                                                                                                           |
|------------------------------|------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `id`                         | `container`      | A unique identifier for the operator.                                                                                                                                                                                                 |
| `format`                     | ``               | The container log format to use if it is known. Users can choose between `docker`, `crio` and `containerd`. If not set, the format will be automatically detected.                                                                    |
| `add_metadata_from_filepath` | `true`           | Set if k8s metadata should be added from the file path. Requires the `log.file.path` field to be present.                                                                                                                             |
| `output`                     | Next in pipeline | The connected operator(s) that will receive all outbound entries.                                                                                                                                                                     |
| `parse_from`                 | `body`           | The [field](../types/field.md) from which the value will be parsed.                                                                                                                                                                   |
| `parse_to`                   | `attributes`     | The [field](../types/field.md) to which the value will be parsed.                                                                                                                                                                     |
| `on_error`                   | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md).                                                                                                                                         |
| `if`                         |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `severity`                   | `nil`            | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator.                                                                                                 |


### Embedded Operations

The `container`  parser can be configured to embed certain operations such as the severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

### Add metadata from file path

Requires `include_file_path: true` in order for the `log.file.path` field to be available for the operator.
If that's not possible, users can disable the metadata addition with `add_metadata_from_filepath: false`.
A file path like `"/var/log/pods/some-ns_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"`,
will produce the following k8s metadata:

```json
{
  "attributes": {
    "k8s": {
      "container": {
        "name": "kube-controller",
        "restart_count": "1"
      }, "pod": {
        "uid": "49cc7c1fd3702c40b2686ea7486091d6",
        "name": "kube-controller-kind-control-plane"
      }, "namespace": {
        "name": "some-ns"
      }
    }
  }
}
```

### Example Configurations:

#### Parse the body as docker container log

Configuration:
```yaml
- type: container
  format: docker
  add_metadata_from_filepath: true
```

Note: in this example the `format: docker` is optional since formats can be automatically detected as well.
      `add_metadata_from_filepath` is true by default as well.

<table>
<tr><td> Input body </td> <td> Output body</td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "{\"log\":\"INFO: log line here\",\"stream\":\"stdout\",\"time\":\"2029-03-30T08:31:20.545192187Z\"}",
  "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
}
```

</td>
<td>

```json
{
  "timestamp": "2024-03-30 08:31:20.545192187 +0000 UTC",
  "body": "log line here",
  "attributes": {
    "time": "2024-03-30T08:31:20.545192187Z", 
    "log.iostream":                "stdout",
    "k8s.pod.name":                "kube-controller-kind-control-plane",
    "k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d6",
    "k8s.container.name":          "kube-controller",
    "k8s.container.restart_count": "1",
    "k8s.namespace.name":          "some",
    "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
  }
}
```

</td>
</tr>
</table>

#### Parse the body as cri-o container log

Configuration:
```yaml
- type: container
```

<table>
<tr><td> Input body </td> <td> Output body</td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "2024-04-13T07:59:37.505201169-05:00 stdout F standalone crio line which is awesome",
  "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
}
```

</td>
<td>

```json
{
  "timestamp": "2024-04-13 12:59:37.505201169 +0000 UTC",
  "body": "standalone crio line which is awesome",
  "attributes": {
    "time": "2024-04-13T07:59:37.505201169-05:00",
    "logtag": "F",
    "log.iostream":                "stdout",
    "k8s.pod.name":                "kube-controller-kind-control-plane",
    "k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d6",
    "k8s.container.name":          "kube-controller",
    "k8s.container.restart_count": "1",
    "k8s.namespace.name":          "some",
    "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
  }
}
```

</td>
</tr>
</table>

#### Parse the body as containerd container log

Configuration:
```yaml
- type: container
```

<table>
<tr><td> Input body </td> <td> Output body</td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "2023-06-22T10:27:25.813799277Z stdout F standalone containerd line that is super awesome",
  "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
}
```

</td>
<td>

```json
{
  "timestamp": "2023-06-22 10:27:25.813799277 +0000 UTC",
  "body": "standalone containerd line that is super awesome",
  "attributes": {
    "time": "2023-06-22T10:27:25.813799277Z",
    "logtag": "F", 
    "log.iostream":                "stdout",
    "k8s.pod.name":                "kube-controller-kind-control-plane",
    "k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d6",
    "k8s.container.name":          "kube-controller",
    "k8s.container.restart_count": "1",
    "k8s.namespace.name":          "some",
    "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
  }
}
```

</td>
</tr>
</table>

#### Parse the multiline as containerd container log and recombine into a single one

Configuration:
```yaml
- type: container
```

<table>
<tr><td> Input body </td> <td> Output body</td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "2023-06-22T10:27:25.813799277Z stdout P multiline containerd line that i",
  "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
},
{
  "timestamp": "",
  "body": "2023-06-22T10:27:25.813799277Z stdout F s super awesomne",
  "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
}
```

</td>
<td>

```json
{
  "timestamp": "2023-06-22 10:27:25.813799277 +0000 UTC",
  "body": "multiline containerd line that is super awesome",
  "attributes": {
    "time": "2023-06-22T10:27:25.813799277Z",
    "logtag": "F",
    "log.iostream":                "stdout",
    "k8s.pod.name":                "kube-controller-kind-control-plane",
    "k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d6",
    "k8s.container.name":          "kube-controller",
    "k8s.container.restart_count": "1",
    "k8s.namespace.name":          "some",
    "log.file.path": "/var/log/pods/some_kube-controller-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d6/kube-controller/1.log"
  }
}
```

</td>
</tr>
</table>