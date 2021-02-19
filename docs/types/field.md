## Fields

A _Field_ is a reference to a value in a log [entry](/docs/types/field.md). 

Many [operators](/docs/operators/README.md) use fields in their configurations. For example, parsers use fields to specify which value to parse and where to write a new value.

Fields are `.`-delimited strings which allow you to select labels or records on the entry. 

Fields can be used to select record, resource, or label values. For values on the record, use the prefix `$record` such as `$record.my_value`. To select a label, prefix your field with `$label` such as with `$label.my_label`. For resource values, use the prefix `$resource`.

If a field contains a dot in it, a field can alternatively use bracket syntax for traversing through a map. For example, to select the key `k8s.cluster.name` on the entry's record, you can use the field `$record["k8s.cluster.name"]`.

Record fields can be nested arbitrarily deeply, such as `$record.my_value.my_nested_value`.

If a field does not start with `$resource`, `$label`, or `$record`, then `$record` is assumed. For example, `my_value` is equivalent to `$record.my_value`.

## Examples

#### Using fields with the restructure operator.

Config:
```yaml
- type: restructure
  ops:
    - add:
        field: "key3"
        value: "value3"
    - remove: "$record.key2.nested_key1"
    - add:
        field: "$labels.my_label"
        value: "my_label_value"
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "labels": {},
  "record": {
    "key1": "value1",
    "key2": {
      "nested_key1": "nested_value1",
      "nested_key2": "nested_value2"
    }
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "labels": {
    "my_label": "my_label_value"
  },
  "record": {
    "key1": "value1",
    "key2": {
      "nested_key2": "nested_value2"
    },
    "key3": "value3"
  }
}
```

</td>
</tr>
</table>


#### Using fields to refer to various values

Given the following entry, we can use fields as follows:

```json
{
  "resource": {
    "uuid": "11112222-3333-4444-5555-666677778888",
  },
  "labels": {
    "env": "prod",
  },
  "record": {
    "message": "Something happened.",
    "details": {
      "count": 100,
      "reason": "event",
    },
  },
}
```

| Field                  | Refers to Value                           |
| ---                    | ---                                       |
| $record.message        | `"Something happened."`                   |
| message                | `"Something happened."`                   |
| $record.details.count  | `100`                                     |
| $labels.env            | `"prod"`                                  |
| $resource.uuid         | `"11112222-3333-4444-5555-666677778888"`  |
