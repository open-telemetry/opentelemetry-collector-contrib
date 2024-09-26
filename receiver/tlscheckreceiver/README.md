# TLS Check Receiver

Emit metrics about x.509 certificates.

## Getting Started

By default, the TLS Check Receiver will emit a single metric, `tlscheck.time_left`, per target. This is measured in seconds until the date and time specified in the `NotAfter` field of the x.509 certificate. 

## Example Configuration

```yaml
receivers:
  tlscheck:
    targets:
      - url: https://example.com
      - url: https://foobar.com:8080
```

## Invalid Certificates and `tlscheck.time_left`

If the certificate is expired, `tlscheck.time_left` will be `0` and the `tlscheck.x509_isvalid` attribute will be `false`.

If the date and time specified in the `NotBefore` field of the x.509 certificate has not yet occured, `tlscheck.time_left` will be reported as a positive integer and `tlscheck.x509_isvalid` attribute will be `false`.

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml).
