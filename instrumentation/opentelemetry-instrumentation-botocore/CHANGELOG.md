# Changelog

## Unreleased

## Version 0.16b0

Released 2020-11-25
- Add propagator injection for botocore calls
  ([#181](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/181))
- Make botocore instrumentation check if instrumentation has been suppressed
  ([#182](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/182))
- Botocore SpanKind as CLIENT and modify existing traced attributes
  ([#150])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/150)

## Version 0.13b0

Released 2020-09-17

- Drop support for Python 3.4
  ([#1099](https://github.com/open-telemetry/opentelemetry-python/pull/1099))

## Version 0.12b0

Released 2020-08-14

- Change package name to opentelemetry-instrumentation-botocore
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))

## Version 0.11b0

Released 2020-07-28

- ext/boto and ext/botocore: fails to export spans via jaeger
([#866](https://github.com/open-telemetry/opentelemetry-python/pull/866))

## 0.9b0

Released 2020-06-10

- Initial release
