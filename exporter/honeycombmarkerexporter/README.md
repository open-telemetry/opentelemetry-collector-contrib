@@ -0,0 +1,18 @@
# Honeycomb Marker Exporter

This exporter allows creating markers, via the Honeycomb Markers API, based on the look of incoming telemetry. 

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `api_url` (Required): You can set the hostname to send marker data to.
* `markers` (Required): This specifies the exact configuration to create an event marker. 
  * `type`: Specifies the marker type. Markers with the same type will appear in the same color in Honeycomb. MarkerType or MarkerColor should be set.  
  * `color`: Specifies the marker color. Will only be used if MarkerType is not set.
  * `messagefield`: This is the attribute that will be used as the message. If necessary the value will be converted to a string.
  * `urlfield`: This is the attribute that will be used as the url. If necessary the value will be converted to a string.
  * `rules`: This is a list of OTTL rules that determine when to create an event marker. 
    * `resourceconditions`: A list of ottlresource conditions that determine a match
    * `logconditions`: A list of ottllog conditions that determine a match
  Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    api_url: "https://api.testhost.io"
    markers:
      - type: "fooType",
        messagefield: "test message",
        urlfield: "https://api.testhost.io",
        rules:
          - resourceconditions:
              - IsMatch(attributes["test"], ".*")
          - logconditions:
              - body == "test"
      - color: "green",
        messagefield: "another test message",
        urlfield: "https://api.testhost.io",
        rules:
          - resourceconditions:
              - IsMatch(attributes["test"], ".*")
          - logconditions:
              - body == "test"
```