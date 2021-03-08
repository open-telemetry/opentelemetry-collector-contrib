## `host_metadata` operator

The `host_metadata` operator adds hostname and ip to the resource of incoming entries.

### Configuration Fields

| Field              | Default          | Description                                                                                                                                                                                                                            |
| ---                | ---              | ---                                                                                                                                                                                                                                    |
| `id`               | `host_metadata`  | A unique identifier for the operator                                                                                                                                                                                                   |
| `output`           | Next in pipeline | The connected operator(s) that will receive all outbound entries                                                                                                                                                                       |
| `include_hostname` | `true`           | Whether to set the `hostname` on the resource of incoming entries                                                                                                                                                                      |
| `include_ip`       | `true`           | Whether to set the `ip` on the resource of incoming entries                                                                                                                                                                            |
| `on_error`         | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md)                                                                                                                                        |
| `if`               |                  | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this parser should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations

#### Add static attributes

Configuration:
```yaml
- type: host_metadata
  include_hostname: true
  include_ip: true
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "record": {
    "message": "test"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "resource": {
    "hostname": "my_host",
    "ip": "0.0.0.0",
  },
  "record": {
    "message": "test"
  }
}
```

</td>
</tr>
</table>
