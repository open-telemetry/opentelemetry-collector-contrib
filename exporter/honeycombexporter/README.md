# Honeycomb Exporter

This exporter supports sending trace data to [Honeycomb](https://www.honeycomb.io). 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `dataset` (Required): The Honeycomb dataset that you want to send events to.
* `api_url` (Optional): You can set the hostname to send events to. Useful for debugging, defaults to `https://api.honeycomb.io`
* `sample_rate` (Optional): Constant sample rate. Can be used to send 1 / x events to Honeycomb. Defaults to 1 (always sample).
* `sample_rate_attribute` (Optional): The name of an attribute that contains the sample_rate for each span. If the attribute is on the span, it takes precedence over the static sample_rate configuration
* `debug` (Optional): Set this to true to get debug logs from the honeycomb SDK. Defaults to false.
Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    dataset: "my-dataset"
    api_url: "https://api.testhost.io"
    sample_rate: 25
    sample_rate_attribute: "hny.sample_rate"
    debug: true
```
