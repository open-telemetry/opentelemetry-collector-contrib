# Resource Detection Processor

Supported pipeline types: metrics, traces

The resource detection processor can be used to detect resource information from the host,
in a format that conforms to the OpenTelemetry resource semantic conventions, and append or
override the resource value in traces and metrics with this information.

Currently supported detectors include:

* Environment Variable: Reads resource information from the `OTEL_RESOURCE` environment
variable. This is expected to be in the format `<key1>=<value1>,<key2>=<value2>,...`, the
details of which are currently pending confirmation in the OpenTelemetry specification.

* GCE Metadata: Uses the [Google Cloud Client Libraries for Go](https://github.com/googleapis/google-cloud-go)
to read resource information from the GCE metadata server as documented by
https://cloud.google.com/compute/docs/storing-retrieving-metadata

## Configuration

```yaml
# a list of resource detectors to run, valid options are: "env", "gce"
detectors: [ <string> ]
# determines if existing resource attributes should be overridden or preserved, defaults to true
override: <bool>
```

The full list of settings exposed for this extension are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).