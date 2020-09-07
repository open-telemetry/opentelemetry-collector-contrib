# Resource Detection Processor

Supported pipeline types: metrics, traces, logs

The resource detection processor can be used to detect resource information from the host,
in a format that conforms to the [OpenTelemetry resource semantic conventions](https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/README.md), and append or
override the resource value in traces and metrics with this information.

Currently supported detectors include:

* Environment Variable: Reads resource information from the `OTEL_RESOURCE` environment
variable. This is expected to be in the format `<key1>=<value1>,<key2>=<value2>,...`, the
details of which are currently pending confirmation in the OpenTelemetry specification.

* GCE Metadata: Uses the [Google Cloud Client Libraries for Go](https://github.com/googleapis/google-cloud-go)
to read resource information from the [GCE metadata server](https://cloud.google.com/compute/docs/storing-retrieving-metadata) to retrieve the following resource attributes:

    * cloud.provider (gcp)
    * cloud.account.id
    * cloud.region
    * cloud.zone
    * host.id
    * host.image.id
    * host.type

* AWS EC2: Uses [AWS SDK for Go](https://docs.aws.amazon.com/sdk-for-go/api/aws/ec2metadata/) to read resource information from the [EC2 instance metadata API](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) to retrieve the following resource attributes:

    * cloud.provider (aws)
    * cloud.account.id
    * cloud.region
    * cloud.zone
    * host.id
    * host.image.id
    * host.type

## Configuration

```yaml
# a list of resource detectors to run, valid options are: "env", "gce", "ec2"
detectors: [ <string> ]
# determines if existing resource attributes should be overridden or preserved, defaults to true
override: <bool>
```

The full list of settings exposed for this extension are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
