# Coralogix Exporter

| Status                   |                        |
| ------------------------ |----------------------- |
| Stability                | traces, metrics [beta] |
|                          | logs [alpha]           |
| Supported pipeline types | traces, metrics, logs  |
| Distributions            | [contrib]              |

The Coralogix exporter sends traces, metrics and logs to [Coralogix](https://coralogix.com/).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security-best-practices.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Configuration

Example configuration:
```yaml
exporters:
  coralogix:
    # The Coralogix traces ingress endpoint
    endpoint: "ingress.coralogix.com:443"
    traces:
      endpoint: "ingress.coralogix.com:443"
    metrics:
      endpoint: "ingress.coralogix.com:443"
    logs:
      endpoint: "ingress.coralogix.com:443"

    # Your Coralogix private key is sensitive
    private_key: "xxx"

    # (Optional) Ordered list of Resource attributes that are used for Coralogix
    # AppName and SubSystem values. The first non-empty Resource attribute is used.
    # Example: application_name_attributes: ["k8s.namespace.name", "service.namespace"]
    # Example: subsystem_name_attributes: ["k8s.deployment.name", "k8s.daemonset.name", "service.name"]
    application_name_attributes:
    - "service.namespace"
    subsystem_name_attributes:
    - "service.name"

    # Traces, Metrics and Logs emitted by this OpenTelemetry exporter 
    # are tagged in Coralogix with the default application and subsystem constants.
    application_name: "MyBusinessEnvironment"
    subsystem_name: "MyBusinessSystem"

    # (Optional) Timeout is the timeout for every attempt to send data to the backend.
    timeout: 30s
```

### Coralogix Single Endpoint

Since v0.76.0 you can specify a single Coralogix endpoint in the configuration file instead of specifying different endpoints for traces, metrics and logs. But you can still specify endpoint per signal type, which allows you to configure gRPC settings per signal type. See [gRPC Configuration Settings documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configgrpc/README.md) for all available settings. For example:

```
    traces:
      endpoint: "ingress.coralogix.com:443"
      read_buffer_size: 524288
    metrics:
      endpoint: "ingress.coralogix.com:443"
    logs:
      endpoint: "ingress.coralogix.com:443"
```

### Coralogix's Endpoints 

Depending on your region, you might need to use a different endpoint. Here are the available Endpoints:

| Region  | Endpoint                        |
|---------|---------------------------------|
| USA1    | `ingress.coralogix.us:443`      |
| APAC1   | `ingress.coralogix.in:443`      |
| APAC2   | `ingress.coralogixsg.com:443`   |
| EUROPE1 | `ingress.coralogix.com:443`     |
| EUROPE2 | `ingress.eu2.coralogix.com:443` |

Additionally, Coralogix supports AWS PrivateLink, which provides private connectivity between virtual private clouds (VPCs), supported AWS services, and your on-premises networks without exposing your traffic to the public internet.

Here are available AWS PrivateLink endpoints:

| Region  | Endpoint                                |
|---------|-----------------------------------------|
| USA1    | `ingress.private.coralogix.com:443`     |
| APAC1   | `ingress.private.coralogix.in:443`       |
| APAC2   | `ingress.private.coralogixsg.com:443`   |
| EUROPE1 | `ingress.private.coralogix.com:443`     |
| EUROPE2 | `ingress.private.eu2.coralogix.com:443` |

Learn more about [AWS PrivateLink in the documentation page](https://coralogix.com/docs/coralogix-amazon-web-services-aws-privatelink-endpoints/).

### Application and SubSystem attributes

v0.62.0 release of OpenTelemetry Collector allows you to map Application name and Subsystem name to Resource attributes. 
You need to set `application_name_attributes` and `subsystem_name_attributes` fields with a list of potential Resource attributes for the AppName and Subsystem values. The first not-empty Resource attribute is going to be used.

### Kubernetes attributes

When using OpenTelemetry Collector with [k8sattribute](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor) processor, you can use attributes coming from Kubernetes, such as `k8s.namespace.name` or `k8s.deployment.name`. The following example shows recommended list of attributes:

```yaml
exporters:
  coralogix:
    # The Coralogix traces ingress endpoint
    endpoint: "ingress.coralogix.com:443"
    application_name_attributes:
      - "service.namespace"
      - "k8s.namespace.name" 
    subsystem_name_attributes:
      - "service.name"
      - "k8s.deployment.name"
      - "k8s.statefulset.name"
      - "k8s.daemonset.name"
      - "k8s.cronjob.name"
      - "k8s.job.name"
      - "k8s.container.name"
```
### Host Attributes

OpenTelemetry Collector [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor) processor can discover Host Resource attributes, such as `host.name` and provide Resource attributes using environment variables, which can be used for setting AppName and SubSystem fields in Coralogix.

Example: 
```yaml
processors:
  resourcedetection/system:
    detectors: ["system", "env"]
    system:
      hostname_sources: ["os"]
```

And setting environment variable such as:
```
OTEL_RESOURCE_ATTRIBUTES="env=production"
```

You can configure Coralogix Exporter:

```yaml
exporters:
  coralogix:
    endpoint: "ingress.coralogix.com:443"
    application_name_attributes:
      - "env" 
    subsystem_name_attributes:
      - "host.name"
```
### EC2 Attributes

OpenTelemetry Collector [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor) processor can discover EC2 Resource attributes, such as EC2 tags as resource attributes.

Example: 
```yaml
processors:
 resourcedetection/ec2:
    detectors: ["ec2"]
    ec2:
      # A list of regex's to match tag keys to add as resource attributes can be specified
      tags:
        - ^ec2.tag.name$
        - ^ec2.tag.subsystem$
```

**_NOTE:_** In order to fetch EC2 tags, the IAM role assigned to the EC2 instance must have a policy that includes the `ec2:DescribeTags` permission.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": "ec2:DescribeTags",
            "Resource": "*"
        }
    ]
}
```

You can configure Coralogix Exporter:

```yaml
exporters:
  coralogix:
    endpoint: "ingress.coralogix.com:443"
    application_name_attributes:
      - "ec2.tag.name" 
    subsystem_name_attributes:
      - "ec2.tag.subsystem"
```

### Custom Attributes

You can combine and create custom Resource attributes using [transform](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor) processor. For example:
```yaml
    transform:
     logs:
       queries:
       - set(resource.attributes["applicationName"], Concat("-", "development-environment", resource.attributes["k8s.namespace.name"]))
```

Then you can use the custom Resource attribute in Coralogix exporter:
```yaml
exporters:
  coralogix:
    endpoint: "ingress.coralogix.com:443"
    application_name_attributes:
      - "applicationName" 
    subsystem_name_attributes:
      - "host.name"
```

### Need help?

Our world-class customer success team is available 24/7 to walk you through the setup for this exporter and answer any questions that may come up.
Feel free to reach out to us **via our in-app chat** or by sending us an email to [support@coralogix.com](mailto:support@coralogix.com).

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
