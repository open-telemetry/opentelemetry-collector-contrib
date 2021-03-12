# Deprecated Stackdriver Exporter

This exporter has been replaced by the [Google Cloud exporter](../googlecloudexporter/README.md).
`stackdriver` exporter configurations will continue to work but are deprecated. Please use [Google
Cloud exporter](../googlecloudexporter/README.md) instead.

`stackdriver` exporter supports the same configuration options as [Google Cloud
exporter](../googlecloudexporter/README.md).

# Recommendations

Please use the [Google Cloud exporter](../googlecloudexporter/README.md) or migrate to it by
changing the exporter entry in your config from `stackdriver` to `googlecloud` and updating your
pipelines to use this new key. All other configuration can rename the same. An example migration
diff might look like this:

```diff
exporters:
- stackdriver:
+ googlecloud
    project: otel-starter-project
    use_insecure: false

processors:
  batch:

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
-     exporters: [stackdriver]
+     exporters: [googlecloud]
```
