# Coralogix Exporter

The Coralogix exporter sends traces to [Coralogix](https://coralogix.com/) as
Coralogix logs.

Supported pipeline types: traces 

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Configuration

Example configuration:
```yaml
exporters:
  coralogix:
    # The Coralogix traces ingress endpoint
    endpoint: "https://api.coralogix.com"

    # Your Coralogix private key is sensitive
    private_key: "xxx"

    # Traces emitted by this OpenTelemetry exporter should be tagged
    # in Coralogix with the following application and subsystem names
    application_name: "MyBusinessEnvironment"
    subsystem_name: "MyBusinessSystem"
```

## Trace Exporter

### Timestamp
Please pay attention to the timestamps that are being produced by the 
receivers that are producing the traces being exported by the Coralogix
exporter. Coralogix can only accept events which are not older than 24 hours.

## Metrics
Prometheus is currently the leading tool for metric collection, it’s easy to integrate and easy to use.

```yaml
exporters:
  prometheusremotewrite:
    # The Coralogix metrics ingress endpoint
    endpoint: "https://api.coralogix.com"

    # Your Coralogix private key (sensitive)
    # This token identifies you into your Coralogix account. 
    # Your Coralogix private key is sensitive!
    headers:
      Authorization: "Bearer <Your Private Key>"
      
    # Mertrics emitted by this OpenTelemetry exporter should be tagged
    # in Coralogix with the following application and subsystem names
    external_labels:
      appLabelName: "MyBusinessEnvironment"
      subSystemLabelName: "MyBusinessSystem"
```

### Need help?
We love to assist our customers, simply [book your implementation session](https://calendly.com/info-coralogix/implementation),
and we will walk you through setting up this exporter, step by step.
