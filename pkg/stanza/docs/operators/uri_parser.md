## `uri_parser` operator

The `uri_parser` operator parses the string-type field selected by `parse_from` as [URI](https://tools.ietf.org/html/rfc3986).

`uri_parser` can handle:
- Absolute URI
  - `https://google.com/v1/app?user_id=2&uuid=57b4dad2-063c-4965-941c-adfd4098face`
- Relative URI
  - `/app?user=admin`
- Query string
  - `?request=681e6fc4-3314-4ccc-933e-4f9c9f0efd24&env=stage&env=dev`
  - Query string must start with a question mark

### Configuration Fields

| Field         | Default          | Description |
| ---           | ---              | ---         |
| `id`          | `uri_parser`     | A unique identifier for the operator. |
| `output`      | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`  | `body`           | The [field](../types/field.md) from which the value will be parsed. |
| `parse_to`    | `attributes`     | The [field](../types/field.md) to which the value will be parsed. |
| `on_error`    | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`          |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Embedded Operations

The `uri_parser` can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

### Output Fields

The following fields are returned. Empty fields are not returned.

| Field  | Type                  | Example                      | Description |
| ---    | ---                   | ---                          | ---         |
| scheme | `string`              | `"http"`                     | [URI Scheme](https://www.iana.org/assignments/uri-schemes/uri-schemes.xhtml). HTTP, HTTPS, FTP, etc. |
| user   | `string`              | `"dev"`                      | [Userinfo](https://tools.ietf.org/html/rfc3986#section-3.2) username. Password is always ignored. |
| host   | `string`              | `"golang.org"`               | The [hostname](https://tools.ietf.org/html/rfc3986#section-3.2.2) such as `www.example.com`, `example.com`, `example`. A scheme is required in order to parse the `host` field. |
| port   | `string`              | `"8443"`                     | The [port](https://tools.ietf.org/html/rfc3986#section-3.2.3) the request is sent to. A scheme is required in order to parse the `port` field. |
| path   | `string`              | `"/v1/app"`                  | URI request [path](https://tools.ietf.org/html/rfc3986#section-3.3). |
| query  | `map[string][]string` | `"query":{"user":["admin"]}` | Parsed URI [query string](https://tools.ietf.org/html/rfc3986#section-3.4). |


### Example Configurations


#### Parse the field `body.message` as absolute URI

Configuration:
```yaml
- type: uri_parser
  parse_from: body.message
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "https://dev:pass@google.com/app?user_id=2&token=001"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "body": {
    "host": "google.com",
    "path": "/app",
    "query": {
      "user_id": [
        "2"
      ],
      "token": [
        "001"
      ]
    },
    "scheme": "https",
    "user": "dev"
  }
}
```

</td>
</tr>
</table>

#### Parse the field `body.message` as relative URI

Configuration:
```yaml
- type: uri_parser
  parse_from: body.message
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "/app?user=admin"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "body": {
    "path": "/app",
    "query": {
      "user": [
        "admin"
      ]
    }
  }
}
```

</td>
</tr>
</table>

#### Parse the field `body.query` as URI query string

Configuration:
```yaml
- type: uri_parser
  parse_from: body.query
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "query": "?request=681e6fc4-3314-4ccc-933e-4f9c9f0efd24&env=stage&env=dev"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "body": {
    "query": {
      "env": [
        "stage",
        "dev"
      ],
      "request": [
        "681e6fc4-3314-4ccc-933e-4f9c9f0efd24"
      ]
    }
  }
}
```

</td>
</tr>
</table>
