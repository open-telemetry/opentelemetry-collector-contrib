# Coralogix Exporter (in development)

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
    # The Coralogix logs ingress endpoint
    # May also be specified with the CORALOGIX_ENDPOINT environment variable
    endpoint: "https://api.coralogix.com"

    # Your Coralogix private key (sensitive)
    # May also be specified with the CORALOGIX_PRIVATE_KEY environment variable
    # Your Coralogix private key is sensitive - you shouldn't define this in the configuration file!
    private_key: "xxx"

    # Traces emitted by this OpenTelemetry exporter should be tagged
    # in Coralogix with the following application and subsystem names
    # May also be specified with the CORALOGIX_APPLICATION_NAME environment variable
    application_name: "MyBusinessSystem"
    # May also be specified with the CORALOGIX_SUBSYSTEM_NAME environment variable
    subsystem_name: "BatchProcessor"
```

## Trace Exporter

### Timestamp
Please pay attention to the timestamps that are being produced by the 
receivers that are producing the traces being exported by the Coralogix
exporter. Coralogix can only accept events which are not older than 24 hours.

### Need help?
We love to assist our customers, simply [book your implementation session](https://calendly.com/info-coralogix/implementation),
and we will walk you through setting up this exporter, step by step.
