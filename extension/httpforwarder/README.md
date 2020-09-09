# HTTP Forwarder Extension

This extension accepts HTTP requests, optionally adds headers to them and forwards them.
The RequestURIs of the original requests are preserved by the extension.

## Configuration

The following settings are required:

* `forward_to`: The host to which requests should be forwarded to.

The following settings are optional:

* `headers`: Additional headers to be added to all requests passing through the extension.
* `http_timeout` (default: `10s`): How long to wait for each request to complete.

### Example

```yaml
  http_forwarder:
    forward_to: http://target/
    headers:
      otel_http_forwarder: dev
    http_timeout: 5s
```
 