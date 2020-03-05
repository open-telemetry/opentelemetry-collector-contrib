# Honeycomb Exporter

This exporter supports sending trace data to [Honeycomb](https://www.honeycomb.io). 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `dataset` (Required): The Honeycomb dataset that you want to send events to.
* `api_url` (Optional): You can set the hostname to send events to. Useful for debugging, defaults to `https://api.honeycomb.io`
Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    dataset: "my-dataset"
    api_url: "https://api.testhost.io"
```
