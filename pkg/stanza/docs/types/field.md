## Fields

A _Field_ is a reference to a value in a log [entry](../types/entry.md).

Many [operators](../operators/README.md) use fields in their configurations. For example, parsers use fields to specify which value to parse and where to write a new value.

Fields are `.`-delimited strings which allow you to select attributes or body on the entry.

Fields can be used to select body, resource, or attribute values. For values on the body, use the prefix `body` such as `body.my_value`. To select an attributes, prefix your field with `attributes` such as with `attributes.my_attribute`. For resource values, use the prefix `resource`.

If a field contains a dot in it, a field can alternatively use bracket syntax for traversing through a map. For example, to select the key `k8s.cluster.name` on the entry's body, you can use the field `body["k8s.cluster.name"]`.

Body fields can be nested arbitrarily deeply, such as `body.my_value.my_nested_value`.

If a field does not start with `resource`, `attributes`, or `body`, then `body` is assumed. For example, `my_value` is equivalent to `body.my_value`.

### Examples

#### Using fields with the add and remove operators.

Config:
```yaml
- type: add
  field: body.key3
  value: val3
- type: remove
  field: body.key2.nested_key1
- type: add
  field: attributes.my_attribute
  value: my_attribute_value
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "attributes": {},
  "body": {
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
  "attributes": {
    "my_attribute": "my_attribute_value"
  },
  "body": {
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
  "attributes": {
    "env": "prod",
  },
  "body": {
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
| body.message        | `"Something happened."`                   |
| message                | `"Something happened."`                   |
| body.details.count  | `100`                                     |
| attributes.env        | `"prod"`                                  |
| resource.uuid         | `"11112222-3333-4444-5555-666677778888"`  |
