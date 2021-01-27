## `elastic_output` operator

The `elastic_output` operator will send entries to an Elasticsearch instance

### Configuration Fields

| Field         | Default          | Description                                                                                           |
| ---           | ---              | ---                                                                                                   |
| `id`          | `elastic_output` | A unique identifier for the operator                                                                  |
| `addresses`   | required         | A list of addresses to send entries to                                                                |
| `username`    |                  | Username for HTTP basic authentication                                                                |
| `password`    |                  | Password for HTTP basic authentication                                                                |
| `cloud_id`    |                  | Endpoint for the Elastic service (https://elastic.co/cloud)                                           |
| `api_key`     |                  | Base64-encoded token for authorization. If set, overrides username and password                       |
| `index_field` | default          | A [field](/docs/types/field.md) that indicates which index to send the log entry to                   |
| `id_field`    |                  | A [field](/docs/types/field.md) that contains an id for the entry. If unset, a unique id is generated |
| `buffer`      |                  | A [buffer](/docs/types/buffer.md) block indicating how to buffer entries before flushing              |
| `flusher`     |                  | A [flusher](/docs/types/flusher.md) block configuring flushing behavior                               |


### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: elastic_output
  addresses:
    - "http://localhost:9200"
  api_key: <my_api_key>
```

#### Configuration with non-default buffer and flusher params

Configuration:
```yaml
- type: elastic_output
  addresses:
    - "http://localhost:9200"
  api_key: <my_api_key>
  buffer:
    type: disk
    path: /tmp/stanza_buffer
  flusher:
    max_concurrent: 8
```
