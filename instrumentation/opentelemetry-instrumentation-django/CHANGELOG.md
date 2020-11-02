# Changelog

## Unreleased

## Version 0.15b0

Released 2020-11-02

- Django instrumentation is now enabled by default but can be disabled by setting `OTEL_PYTHON_DJANGO_INSTRUMENT` to `False`
  ([#1239](https://github.com/open-telemetry/opentelemetry-python/pull/1239))
- Bugfix use request.path replace request.get_full_path(). It will get correct span name
  ([#1309](https://github.com/open-telemetry/opentelemetry-python/pull/1309#))
- Record span status and http.status_code attribute on exception 
  ([#1257](https://github.com/open-telemetry/opentelemetry-python/pull/1257))
- Added capture of http.route
  ([#1226](https://github.com/open-telemetry/opentelemetry-python/issues/1226))
- Add support for tracking http metrics
  ([#1230](https://github.com/open-telemetry/opentelemetry-python/pull/1230))

## Version 0.14b0

Released 2020-10-13

- Changed span name extraction from request to comply semantic convention ([#992](https://github.com/open-telemetry/opentelemetry-python/pull/992)) 
- Added support for `OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS` ([#1154](https://github.com/open-telemetry/opentelemetry-python/pull/1154))

## Version 0.13b0

Released 2020-09-17

- Drop support for Python 3.4
  ([#1099](https://github.com/open-telemetry/opentelemetry-python/pull/1099))

## Version 0.12b0

Released 2020-08-14

- Change package name to opentelemetry-instrumentation-django
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- Update environment variable names, prefix changed from `OPENTELEMETRY` to `OTEL` ([#904](https://github.com/open-telemetry/opentelemetry-python/pull/904))

## Version 0.11b0

Released 2020-07-28

- Use one general exclude list instead of two ([#872](https://github.com/open-telemetry/opentelemetry-python/pull/872))

## 0.8b0

Released 2020-05-27

- Add exclude list for paths and hosts to prevent from tracing
  ([#670](https://github.com/open-telemetry/opentelemetry-python/pull/670))
- Add support for django >= 1.10 (#717)

## 0.7b1

Released 2020-05-12

- Initial release
