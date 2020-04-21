# Honeycomb Exporter

This exporter supports sending trace data to [Honeycomb](https://www.honeycomb.io). 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account. It is also possible to 
  configure the API key via the `HONEYCOMB_WRITE_KEY` environment variable, but note that the value in the configuration 
  file will override the environment variable.
* `dataset` (Required): The Honeycomb dataset that you want to send events to. It is also possible to configure the 
  dataset via the `HONEYCOMB_DATASET` environment variable, but note that the value in the configuration file will 
  override the environment variable.
* `api_url` (Optional): You can set the hostname to send events to. Useful for debugging, defaults to `https://api.honeycomb.io`
Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    dataset: "my-dataset"
    api_url: "https://api.testhost.io"
```
