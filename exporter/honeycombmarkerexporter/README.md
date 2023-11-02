@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter allows creating markers, via the Honeycomb Markers API, based on the look of incoming telemetry. 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `api_url` (Required): You can set the hostname to send marker data to.
* `markers` (Required): This specifies the exact configuration to create an event marker. 
  * `type`: Specifies the marker type. Markers with the same type will appear in the same color in Honeycomb.  
  * `message_key`: This is the attribute that will be used as the message. If necessary the value will be converted to a string.
  * `url_key`: This is the attribute that will be used as the url. If necessary the value will be converted to a string.
  * `rules`: This is a list of OTTL rules that determine when to create an event marker. 
    * `log_conditions`: A list of ottllog conditions that determine a match
  Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    api_url: "https://api.honeycomb.io:443"
    markers:
      - type: ""
        message_key: "test message"
        url_key: "https://api.testhost.io"
        dataset_slug: "__all__"
        rules:
          - logconditions:
              - body == "test"
```