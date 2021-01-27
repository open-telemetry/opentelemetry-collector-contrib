## `foward_input` operator

The `foward_input` operator receives logs from another Stanza instance running `forward_output`.

### Configuration Fields

| Field            | Default          | Description                                           |
| ---              | ---              | ---                                                   |
| `id`             | `forward_output` | A unique identifier for the operator                  |
| `listen_address` | `:80`            | The IP address and port to listen on                  |
| `tls`            |                  | A block for configuring the server to listen with TLS |

#### TLS block configuration

| Field       | Default | Description                          |
| ---         | ---     | ---                                  |
| `cert_file` |         | The location of the certificate file |
| `key_file`  |         | The location of the key file         |


### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: forward_input
  listen_address: ":25535"
```

#### TLS configuration

Configuration:
```yaml
- type: forward_input
  listen_address: ":25535"
  tls:
    cert_file: /tmp/public.crt
    key_file: /tmp/private.key
```
