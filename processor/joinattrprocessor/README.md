# Join attributes processor

Supported pipeline types: traces

This processor join values of chosen attributes into one attribute using a chosen separator. If the target attribute already exists it is not overriden by default, but it can be configured.

## Processor Configuration

Full proccessor configuration is given in the following example

```yaml
processors:
  join_attributes:
    # An array of attribute keys whose values are to be joined.
    attributes: ["attribute-1",...,"attribute-N"]
    # A key of an attribute that will be populated with the new value.
    target_attribute: "some-attribute"
    # A character/string to separate the values.
    separator: ","

  join_attributes/override:
    # An array of attribute keys whose values are to be joined.
    attributes: ["attribute-1",...,"attribute-N"]
    # A key of an attribute that will be populated with the new value.
    target_attribute: "some-attribute"
    # A character/string to separate the values.
    separator: ","
    # If the target attribute already has a value, override it. (default is false)
    override: true
```

Refer to [config.yaml](./testdata/config.yaml) for how to fit the configuration
into an OpenTelemetry Collector pipeline definition.