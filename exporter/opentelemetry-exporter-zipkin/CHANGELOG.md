# Changelog

## Unreleased

- Add support for OTEL_EXPORTER_ZIPKIN_ENDPOINT env var. As part of this change, the 
  configuration of the ZipkinSpanExporter exposes a `url` argument to replace `host_name`,
  `port`, `protocol`, `endpoint`. This brings this implementation inline with other
  implementations. 
  ([#1064](https://github.com/open-telemetry/opentelemetry-python/pull/1064))
- Zipkin exporter report instrumentation info. 
  ([#1097](https://github.com/open-telemetry/opentelemetry-python/pull/1097))  

## Version 0.12b0

Released 2020-08-14

- Change package name to opentelemetry-exporter-zipkin
  ([#953](https://github.com/open-telemetry/opentelemetry-python/pull/953))
- Add proper length zero padding to hex strings of traceId, spanId, parentId sent on the wire, for compatibility with jaeger-collector
  ([#908](https://github.com/open-telemetry/opentelemetry-python/pull/908))

## 0.8b0

Released 2020-05-27

- Transform resource to tags when exporting
  ([#707](https://github.com/open-telemetry/opentelemetry-python/pull/707))

## 0.7b1

Released 2020-05-12

- bugfix: 'debug' field is now correct
  ([#549](https://github.com/open-telemetry/opentelemetry-python/pull/549))

## 0.4a0

Released 2020-02-21

- Initial release
