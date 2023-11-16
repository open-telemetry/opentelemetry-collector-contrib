@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter allows creating [markers](https://docs.honeycomb.io/working-with-your-data/markers/), via the [Honeycomb Markers API](https://docs.honeycomb.io/api/tag/Markers#operation/createMarker), based on the look of incoming telemetry. 

The following configuration options are supported:

* `api_key` (Required): This is the API key for your Honeycomb account.
* `api_url` (Optional): This sets the hostname to send marker data to. If not set, will default to `https://api.honeycomb.io/`
* `markers` (Required): This is a list of configurations to create an event marker. 
  * `type` (Required): Specifies the marker type.
  * `dataset_slug` (Optional): The dataset in which to create the marker. If not set, will default to `__all__`.
  * `message_key` (Optional): This attribute will be used as the message. It describes the event marker. If necessary the value will be converted to a string.
  * `url_key` (Optional): This attribute will be used as the url. It can be accessed through the event marker in Honeycomb. If necessary the value will be converted to a string.
  * `rules` (Required): This is a list of [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl) rules that determine when to create an event marker. 
    * `log_conditions` (Required): A list of [OTTL log](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottllog) conditions that determine a match. The marker will be created if **ANY** condition matches.

Example:
```yaml
exporters:
  honeycombmarker:
    api_key: {{env:HONEYCOMB_API_KEY}}
    markers:
      # Creates a new marker anytime the exporter sees a k8s event with a reason of Backoff
      - type: k8s-backoff-events
        rules:
          - log_conditions:
              - IsMap(body) and IsMap(body["object"] and body["object"]["reason"] == "Backoff"
```

