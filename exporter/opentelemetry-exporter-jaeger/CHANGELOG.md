# Changelog

## Unreleased

## Version 0.13b0

Released 2020-09-17
- Drop support for Python 3.4
  ([#1099](https://github.com/open-telemetry/opentelemetry-python/pull/1099))
- Report instrumentation info
  ([#1098](https://github.com/open-telemetry/opentelemetry-python/pull/1098))

## Version 0.12b0

Released 2020-08-14

- Change package name to opentelemetry-exporter-jaeger
  ([#953](https://github.com/open-telemetry/opentelemetry-python/pull/953))

- Thrift URL for Jaeger exporter doesn't allow HTTPS (hardcoded to HTTP)
  ([#978] (https://github.com/open-telemetry/opentelemetry-python/pull/978))

## 0.8b0

Released 2020-05-27

- Transform resource to tags when exporting
  ([#645](https://github.com/open-telemetry/opentelemetry-python/pull/645))
- ext/boto: Could not serialize attribute aws.region to tag when exporting via jaeger
  Serialize tuple type values by coercing them into a string, since Jaeger does not
  support tuple types.
  ([#865](https://github.com/open-telemetry/opentelemetry-python/pull/865))

## 0.6b0

Released 2020-03-30

- Exporting to collector now works
  ([#508](https://github.com/open-telemetry/opentelemetry-python/pull/508))

## 0.4a0

Released 2020-02-21

- Export span status ([#367](https://github.com/open-telemetry/opentelemetry-python/pull/367))
- Export span kind ([#387](https://github.com/open-telemetry/opentelemetry-python/pull/387))

## 0.3a0

Released 2019-12-11

## 0.2a0

Released 2019-10-29

- Initial release
