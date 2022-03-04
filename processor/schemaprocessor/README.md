# Schema Processor

Supported pipeline types: traces, metrics, logs.

The schema processor modifies transforms the schema of the input data from one version to
another. Please refer to [config.go](./config.go) for the config spec.


```yaml
processors:
  schema:
    transform:
      # A set of one or more transform rules.
        # "from" defines the Schema URL to match the input data. Must be either a full
        # Schema URL with version number or a wildcard URL where version number can be partial.  
      - from: https://opentelemetry.io/schemas/1.*
        # "to" defines the Schema URL to transform the input data to. MUST belong to the
        # same Schema Family as the "from" setting. MUST be a Schema URL with a specific
        # version number, wildcards are not allowed.
        to: https://opentelemetry.io/schemas/1.9.0

extensions:
  # Optional storage where schema files can be cached. schema processor will automatically
  # use a storage extension if it detects one. 
  file_storage:
    directory: /var/lib/otelcol/mydir
```

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.
