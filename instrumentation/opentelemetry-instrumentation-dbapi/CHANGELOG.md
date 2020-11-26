# Changelog

## Unreleased

## Version 0.16b0

Released 2020-11-25

- Update dbapi and its dependant instrumentations to follow semantic conventions
  ([#195](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/195))

- Stop capturing query parameters by default
  ([#156](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/156))

## Version 0.13b0

Released 2020-09-17

- bugfix: cursors and connections now produce spans when used with context managers
  ([#1028](https://github.com/open-telemetry/opentelemetry-python/pull/1028))
- Drop support for Python 3.4
  ([#1099](https://github.com/open-telemetry/opentelemetry-python/pull/1099))

## Version 0.12b0

Released 2020-08-14

- Change package name to opentelemetry-instrumentation-dbapi
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))

## 0.7b1

Released 2020-05-12

- Implement instrument_connection and uninstrument_connection ([#624](https://github.com/open-telemetry/opentelemetry-python/pull/624))

## 0.4a0

Released 2020-02-21

- Initial release
