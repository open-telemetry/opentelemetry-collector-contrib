@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter allows creating markers, via the Honeycomb Markers API, based on the look of incoming telemetry. 

The following configuration options are supported:

* `api_key` (Required): This is the API key for your Honeycomb account.
* `api_url` (Required): This sets the hostname to send marker data to.
* `markers` (Required): This specifies the exact configuration to create an event marker. 
  * `type` (Required): Specifies the marker type. 
  * `message_key`: This attribute will be used as the message. If necessary the value will be converted to a string.
  * `url_key`: This attribute will be used as the url. If necessary the value will be converted to a string.
  * `rules` (Required): This is a list of OTTL rules that determine when to create an event marker. 
    * `log_conditions` (Required): A list of ottllog conditions that determine a match
  Example:

```yaml
exporters:
  honeycombmarker:
    api_key: "my-api-key"
    api_url: "https://api.honeycomb.io:443"
    markers:
      - type: "test-type"
        message_key: "test message"
        url_key: "https://api.testhost.io"
        dataset_slug: "__all__"
        rules:
          - log_conditions:
              - body == "test"
```