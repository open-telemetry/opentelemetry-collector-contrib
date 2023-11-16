@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter allows creating markers, via the Honeycomb Markers API, based on the look of incoming telemetry. 

The following configuration options are supported:

* `api_key` (Required): This is the API key for your Honeycomb account.
* `api_url` (Required): This sets the hostname to send marker data to.
* `markers` (Required): This is a list of configurations to create an event marker. 
  * `type` (Required): Specifies the marker type. 
  * `message_key`: This attribute will be used as the message. It describes the event marker. If necessary the value will be converted to a string.
  * `url_key`: This attribute will be used as the url. It can be accessed through the event marker in Honeycomb. If necessary the value will be converted to a string.
  * `rules` (Required): This is a list of OTTL rules that determine when to create an event marker. 
    * `log_conditions` (Required): A list of ottllog conditions that determine a match
  Example:

```yaml
exporters:
  honeycombmarker:
    api_key: "environment-api-key"
    api_url: "https://api.honeycomb.io"
    markers:
      - type: "marker-type"
        message_key: "marker-message"
        url_key: "marker-url"
        dataset_slug: "__all__"
        rules:
          - log_conditions:
              - body == "test"
```