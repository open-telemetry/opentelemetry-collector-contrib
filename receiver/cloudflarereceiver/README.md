# Cloudflare Receiver

| Status                   |           |
|--------------------------|-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |


This Cloudflare receiver allows Cloudflare's [LogPush Jobs](https://developers.cloudflare.com/logs/logpush/) to send logs over HTTPS from the Cloudflare logs aggregation system to an OpenTelemetry collector.

## Getting Started

To successfully operate this receiver, you must follow these steps in order:
1. Receive a properly CA signed SSL certificate for use on the collector host.
2. Configure the receiver using the previously acquired SSL certificate, and then start the collector.
3. Create a LogPush HTTP destination job following the [directions](https://developers.cloudflare.com/logs/get-started/enable-destinations/http/) provided by Cloudflare. When the job is created, it will attempt to validate the connection to the receiver.
   - If you've configured the receiver with a `secret` to validate requests, ensure you add the value to the `destination_conf` parameter of the LogPush job by adding its value as a query parameter under the `header_X-CF-Secret` parameter. For example, `"destination_conf": "https://example.com?header_X-CF-Secret=abcd1234"`.
4. If the LogPush job creates successfully, the receiver is correctly configured and the LogPush job will send it metrics. If not, the most likely issue is with the SSL configuration, check both the LogPush API response and the receiver's logs for more details.

## Configuration

- `tls` (Cloudflare requires TLS, and self-signed will not be sufficient)
    - `cert_file` 
       - Be sure to append your CA certificate to the server's certificate
    - `key_file`
- `endpoint` 
  - The endpoint on which the receiver will await requests from Cloudflare
- `secret`
  - If this value is set, the receiver expects to see it in any valid requests under the `X-CF-Secret` header
- `timestamp_field` (default: `EdgeStartTimestamp`)
    - This receiver was built with the `http_requests` dataset in mind, but should be able to support any Cloudflare dataset. If using another dataset, you will need to set the `timestamp_field` appropriately in order to have the log record be associated with the correct timestamp.

### Example:

```yaml
receivers:
  cloudflare:
    tls:
      key_file: some_key_file
      cert_file: some_cert_file
    endpoint: 0.0.0.0:12345
    secret: 1234567890abcdef1234567890abcdef
    timestamp_field: EdgeStartTimestamp
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
