## Fields

_Fields_ are the primary way to tell stanza which values of an entry to use in its operators.
Most often, these will be things like fields to parse for a parser operator, or the field to write a new value to.

Fields are `.`-delimited strings which allow you to select labels or records on the entry. Fields can currently be used to select labels, values on a record, or resource values. To select a label, prefix your field with `$label` such as with `$label.my_label`. For values on the record, use the prefix `$record` such as `$record.my_value`. For resource values, use the prefix `$resource`.

If a key contains a dot in it, a field can alternatively use bracket syntax for traversing through a map. For example, to select the key `k8s.cluster.name` on the entry's record, you can use the field `$record["k8s.cluster.name"]`.

Record fields can be nested arbitrarily deeply, such as `$record.my_value.my_nested_value`.

If a field does not start with either `$label` or `$record`, `$record` is assumed. For example, `my_value` is equivalent to `$record.my_value`.

## Examples

Using fields with the restructure operator.

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
