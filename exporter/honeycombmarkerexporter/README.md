@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter supports sending creating markers and sending them to the Honeycomb Markers API. 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `api_url` (Required): You can set the hostname to send marker data to.
* `presets` (Required): This exporter will only allow preset configurations to start. 
  Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    api_url: "https://api.testhost.io"
    presets: True 
```