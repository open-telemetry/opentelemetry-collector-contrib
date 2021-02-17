# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/open-telemetry/opentelemetry-python-contrib/compare/v0.18b0...HEAD)

## [0.18b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.18b0) - 2021-02-16

### Added
- `opentelemetry-propagator-ot-trace` Add OT Trace Propagator
  ([#302](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/302))

### Removed
- Remove `component` span attribute in instrumentations. 
  `opentelemetry-instrumentation-aiopg`, `opentelemetry-instrumentation-dbapi` Remove unused `database_type` parameter from `trace_integration` function.
  ([#301](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/301))
- `opentelemetry-instrumentation-asgi` Return header values using case insensitive keys
  ([#308](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/308))
- Remove metrics from all instrumentations
  ([#312](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/312))
- `opentelemetry-instrumentation-boto` updated to set span attributes instead of overriding the resource.
  ([#310](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/310))
- `opentelemetry-instrumentation-grpc` Fix issue tracking child spans in streaming responses
  ([#260](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/260))
- `opentelemetry-instrumentation-grpc` Updated client attributes, added tests, fixed examples, docs
  ([#269](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/269))

## [0.17b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.17b0) - 2021-01-20

### Added
- `opentelemetry-instrumentation-sqlalchemy` Ensure spans have kind set to "CLIENT"
  ([#278](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/278))
- `opentelemetry-instrumentation-celery` Add support for Celery version 5.x
  ([#266](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/266))
- `opentelemetry-instrumentation-urllib` Add urllib instrumentation
  ([#222](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/222))
- `opentelemetry-exporter-datadog` Add fields method
  ([#226](https://github.com/open-telemetry/opentelemetry-python/pull/226))
- `opentelemetry-sdk-extension-aws` Add method to return fields injected by propagator
  ([#226](https://github.com/open-telemetry/opentelemetry-python/pull/226))
- `opentelemetry-exporter-prometheus-remote-write` Prometheus Remote Write Exporter Setup
  ([#180](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/180))
- `opentelemetry-exporter-prometheus-remote-write` Add Exporter constructor validation methods in Prometheus Remote Write Exporter
  ([#206](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/206))
- `opentelemetry-exporter-prometheus-remote-write` Add conversion to TimeSeries methods in Prometheus Remote Write Exporter
  ([#207](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/207))
- `opentelemetry-exporter-prometheus-remote-write` Add request methods to Prometheus Remote Write Exporter
  ([#212](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/212))
- `opentelemetry-instrumentation-fastapi` Added support for excluding some routes with env var `OTEL_PYTHON_FASTAPI_EXCLUDED_URLS`
  ([#237](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/237))
- `opentelemetry-instrumentation-starlette` Added support for excluding some routes with env var `OTEL_PYTHON_STARLETTE_EXCLUDED_URLS`
  ([#237](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/237))
- Add Prometheus Remote Write Exporter integration tests in opentelemetry-docker-tests
  ([#216](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/216))
- `opentelemetry-instrumentation-grpc` Add tests for grpc span attributes, grpc `abort()` conditions
  ([#236](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/236))
- Add README and example app for Prometheus Remote Write Exporter
  ([#227](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/227]))
- `opentelemetry-instrumentation-botocore` Adds a field to report the number of retries it take to complete an API call
  ([#275](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/275))
- `opentelemetry-instrumentation-requests` Use instanceof to check if responses are valid Response objects
  ([#273](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/273))

### Changed
- `opentelemetry-instrumentation-asgi`, `opentelemetry-instrumentation-wsgi` Return `None` for `CarrierGetter` if key not found
  ([#233](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/233))
- `opentelemetry-instrumentation-grpc` Comply with updated spec, rework tests
  ([#236](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/236))
- `opentelemetry-instrumentation-asgi`, `opentelemetry-instrumentation-falcon`, `opentelemetry-instrumentation-flask`, `opentelemetry-instrumentation-pyramid`, `opentelemetry-instrumentation-wsgi` Renamed `host.port` attribute to `net.host.port`
  ([#242](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/242))
- `opentelemetry-instrumentation-flask` Do not emit a warning message for request contexts created with `app.test_request_context`
  ([#253](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/253))
- `opentelemetry-instrumentation-requests`, `opentelemetry-instrumentation-urllib` Fix span name callback parameters
  ([#259](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/259))
- `opentelemetry-exporter-datadog` Fix unintentional type change of span trace flags
  ([#261](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/261))
- `opentelemetry-instrumentation-aiopg` Fix AttributeError `__aexit__` when `aiopg.connect` and `aio[g].create_pool` used with async context manager
  ([#235](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/235))
- `opentelemetry-exporter-datadog` `opentelemetry-sdk-extension-aws` Fix reference to ids_generator in sdk
  ([#283](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/283))
- `opentelemetry-instrumentation-sqlalchemy` Use SQL operation and DB name as span name.
  ([#254](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/254))
- `opentelemetry-instrumentation-dbapi`, `TracedCursor` replaced by `CursorTracer`
  ([#246](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/246))
- `opentelemetry-instrumentation-psycopg2`, Added support for psycopg2 registered types.
  ([#246](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/246))
- `opentelemetry-instrumentation-dbapi`, `opentelemetry-instrumentation-psycopg2`, `opentelemetry-instrumentation-mysql`, `opentelemetry-instrumentation-pymysql`, `opentelemetry-instrumentation-aiopg` Use SQL command name as the span operation name instead of the entire query.
  ([#246](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/246))
- Update TraceState to adhere to specs
  ([#276](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/276))

### Removed
- Remove Configuration
  ([#285](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/285))

## [0.16b1](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.16b1) - 2020-11-26

## [0.16b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.16b0) - 2020-11-25

### Added
- `opentelemetry-instrumentation-flask` Add span name callback
  ([#152](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/152))
- `opentelemetry-sdk-extension-aws` Add AWS X-Ray Ids Generator Entry Point
  ([#201](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/201))
- `opentelemetry-sdk-extension-aws` Fix typo for installing OTel SDK in docs
  ([#200](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/200))
- `opentelemetry-sdk-extension-aws` Import missing components for docs
  ([#198](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/198))
- `opentelemetry-sdk-extension-aws` Provide components needed to Configure OTel SDK for Tracing with AWS X-Ray
  ([#130](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/130))
- `opentelemetry-instrumentation-sklearn` Initial release
  ([#151](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/151))
- `opentelemetry-instrumentation-requests` Add span name callback
  ([#158](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/158))
- `opentelemetry-instrumentation-botocore` Add propagator injection for botocore calls
  ([#181](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/181))

### Changed
- `opentelemetry-instrumentation-pymemcache` Update pymemcache instrumentation to follow semantic conventions
  ([#183](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/183))
- `opentelemetry-instrumentation-redis` Update redis instrumentation to follow semantic conventions
  ([#184](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/184))
- `opentelemetry-instrumentation-pymongo` Update pymongo instrumentation to follow semantic conventions
  ([#203](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/203))
- `opentelemetry-instrumentation-sqlalchemy` Update sqlalchemy instrumentation to follow semantic conventions
  ([#202](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/202))
- `opentelemetry-instrumentation-botocore` Make botocore instrumentation check if instrumentation has been suppressed
  ([#182](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/182))
- `opentelemetry-instrumentation-botocore` Botocore SpanKind as CLIENT and modify existing traced attributes
  ([#150])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/150)
- `opentelemetry-instrumentation-dbapi` Update dbapi and its dependant instrumentations to follow semantic conventions
  ([#195](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/195))
- `opentelemetry-instrumentation-dbapi` Stop capturing query parameters by default
  ([#156](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/156))
- `opentelemetry-instrumentation-asyncpg` Update asyncpg instrumentation to follow semantic conventions
  ([#188](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/188))
- `opentelemetry-instrumentation-grpc` Update protobuf versions
  ([#1356](https://github.com/open-telemetry/opentelemetry-python/pull/1356))

## [0.15b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.15b0) - 2020-11-02

### Added
- `opentelemetry-instrumentation-requests` Add support for tracking http metrics
  ([#1230](https://github.com/open-telemetry/opentelemetry-python/pull/1230))
- `opentelemetry-instrumentation-django` Added capture of http.route
  ([#1226](https://github.com/open-telemetry/opentelemetry-python/issues/1226))
- `opentelemetry-instrumentation-django` Add support for tracking http metrics
  ([#1230](https://github.com/open-telemetry/opentelemetry-python/pull/1230))

### Changed
- `opentelemetry-exporter-datadog` Make `SpanProcessor.on_start` accept parent Context
  ([#1251](https://github.com/open-telemetry/opentelemetry-python/pull/1251))
- `opentelemetry-instrumentation-flask` Use `url.rule` instead of `request.endpoint` for span name
  ([#1260](https://github.com/open-telemetry/opentelemetry-python/pull/1260))
- `opentelemetry-instrumentation-django` Django instrumentation is now enabled by default but can be disabled by setting `OTEL_PYTHON_DJANGO_INSTRUMENT` to `False`
  ([#1239](https://github.com/open-telemetry/opentelemetry-python/pull/1239))
- `opentelemetry-instrumentation-django` Bugfix use request.path replace request.get_full_path(). It will get correct span name
  ([#1309](https://github.com/open-telemetry/opentelemetry-python/pull/1309#))
- `opentelemetry-instrumentation-django` Record span status and http.status_code attribute on exception
  ([#1257](https://github.com/open-telemetry/opentelemetry-python/pull/1257))
- `opentelemetry-instrumentation-grpc` Rewrite gRPC server interceptor
  ([#1171](https://github.com/open-telemetry/opentelemetry-python/pull/1171))

## [0.14b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.14b0) - 2020-10-13

### Added
- `opentelemetry-exporter-datadog` Add support for span resource labels and service name
- `opentelemetry-instrumentation-celery` Span operation names now include the task type.
  ([#1135](https://github.com/open-telemetry/opentelemetry-python/pull/1135))
- `opentelemetry-instrumentation-celery` Added automatic context propagation.
  ([#1135](https://github.com/open-telemetry/opentelemetry-python/pull/1135))
- `opentelemetry-instrumentation-falcon` Added support for `OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS`
  ([#1158](https://github.com/open-telemetry/opentelemetry-python/pull/1158))
- `opentelemetry-instrumentation-tornado` Added support for `OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS`
  ([#1178](https://github.com/open-telemetry/opentelemetry-python/pull/1178))
- `opentelemetry-instrumentation-django` Added support for `OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS`
  ([#1154](https://github.com/open-telemetry/opentelemetry-python/pull/1154))

### Changed
- `opentelemetry-instrumentation-pymongo` Cast PyMongo commands as strings
  ([#1132](https://github.com/open-telemetry/opentelemetry-python/pull/1132))
- `opentelemetry-instrumentation-system-metrics` Fix issue when specific metrics are not available in certain OS
  ([#1207](https://github.com/open-telemetry/opentelemetry-python/pull/1207))
- `opentelemetry-instrumentation-pymysql` Bumped version from 0.9.3 to 0.10.1
  ([#1228](https://github.com/open-telemetry/opentelemetry-python/pull/1228))
- `opentelemetry-instrumentation-django` Changed span name extraction from request to comply semantic convention
  ([#992](https://github.com/open-telemetry/opentelemetry-python/pull/992))

## [0.13b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.13b0) - 2020-09-17

### Added
- `opentelemetry-instrumentation-falcon` Initial release. Added instrumentation for Falcon 2.0+
- `opentelemetry-instrumentation-tornado` Initial release. Supports Tornado 6.x on Python 3.5 and newer.
- `opentelemetry-instrumentation-aiohttp-client` Add instrumentor and auto instrumentation support for aiohttp
  ([#1075](https://github.com/open-telemetry/opentelemetry-python/pull/1075))
- `opentelemetry-instrumentation-requests` Add support for instrumenting prepared requests
  ([#1040](https://github.com/open-telemetry/opentelemetry-python/pull/1040))
- `opentelemetry-instrumentation-requests` Add support for http metrics
  ([#1116](https://github.com/open-telemetry/opentelemetry-python/pull/1116))

### Changed
- `opentelemetry-instrumentation-aiohttp-client` Updating span name to match semantic conventions
  ([#972](https://github.com/open-telemetry/opentelemetry-python/pull/972))
- `opentelemetry-instrumentation-dbapi` cursors and connections now produce spans when used with context managers
  ([#1028](https://github.com/open-telemetry/opentelemetry-python/pull/1028))

### Removed
- Drop support for Python 3.4
  ([#1099](https://github.com/open-telemetry/opentelemetry-python/pull/1099))

## [0.12b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.12.0) - 2020-08-14

### Changed
- `opentelemetry-ext-pymemcache` Change package name to opentelemetry-instrumentation-pymemcache
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-redis` Update default SpanKind to `SpanKind.CLIENT`
  ([#965](https://github.com/open-telemetry/opentelemetry-python/pull/965))
- `opentelemetry-ext-redis` Change package name to opentelemetry-instrumentation-redis
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-datadog` Change package name to opentelemetry-exporter-datadog
  ([#953](https://github.com/open-telemetry/opentelemetry-python/pull/953))
- `opentelemetry-ext-jinja2` Change package name to opentelemetry-instrumentation-jinja2
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-elasticsearch` Update environment variable names, prefix changed from `OPENTELEMETRY` to `OTEL`
  ([#904](https://github.com/open-telemetry/opentelemetry-python/pull/904))
- `opentelemetry-ext-elasticsearch` Change package name to opentelemetry-instrumentation-elasticsearch
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-celery` Change package name to opentelemetry-instrumentation-celery
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-pyramid` Change package name to opentelemetry-instrumentation-pyramid
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-pyramid` Update environment variable names, prefix changed from `OPENTELEMETRY` to `OTEL`
  ([#904](https://github.com/open-telemetry/opentelemetry-python/pull/904))
- `opentelemetry-ext-pymongo` Change package name to opentelemetry-instrumentation-pymongo
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-sqlite3` Change package name to opentelemetry-instrumentation-sqlite3
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-sqlalchemy` Change package name to opentelemetry-instrumentation-sqlalchemy
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-psycopg2` Change package name to opentelemetry-instrumentation-psycopg2
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-aiohttp-client` Change package name to opentelemetry-instrumentation-aiohttp-client
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-boto` Change package name to opentelemetry-instrumentation-boto
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-system-metrics` Change package name to opentelemetry-instrumentation-system-metrics
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-asgi` Change package name to opentelemetry-instrumentation-asgi
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-wsgi` Change package name to opentelemetry-instrumentation-wsgi
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-pymysql` Change package name to opentelemetry-instrumentation-pymysql
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-requests` Change package name to opentelemetry-instrumentation-requests
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-requests` Span name reported updated to follow semantic conventions to reduce
  cardinality ([#972](https://github.com/open-telemetry/opentelemetry-python/pull/972))
- `opentelemetry-ext-botocore` Change package name to opentelemetry-instrumentation-botocore
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))
- `opentelemetry-ext-dbapi` Change package name to opentelemetry-instrumentation-dbapi
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-flask` Change package name to opentelemetry-instrumentation-flask
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-flask` Update environment variable names, prefix changed from `OPENTELEMETRY` to `OTEL`
  ([#904](https://github.com/open-telemetry/opentelemetry-python/pull/904))
- `opentelemetry-ext-django` Change package name to opentelemetry-instrumentation-django
  ([#961](https://github.com/open-telemetry/opentelemetry-python/pull/961))
- `opentelemetry-ext-django` Update environment variable names, prefix changed from `OPENTELEMETRY` to `OTEL`
  ([#904](https://github.com/open-telemetry/opentelemetry-python/pull/904))
- `opentelemetry-ext-asyncpg` Change package name to opentelemetry-instrumentation-asyncpg
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-mysql` Change package name to opentelemetry-instrumentation-mysql
  ([#966](https://github.com/open-telemetry/opentelemetry-python/pull/966))
- `opentelemetry-ext-grpc` Change package name to opentelemetry-instrumentation-grpc
  ([#969](https://github.com/open-telemetry/opentelemetry-python/pull/969))

## [0.11b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.11.0) - 2020-07-28

### Added
- `opentelemetry-instrumentation-aiopg` Initial release
- `opentelemetry-instrumentation-fastapi` Initial release
  ([#890](https://github.com/open-telemetry/opentelemetry-python/pull/890))
- `opentelemetry-ext-grpc` Add status code to gRPC client spans
  ([896](https://github.com/open-telemetry/opentelemetry-python/pull/896))
- `opentelemetry-ext-grpc` Add gRPC client and server instrumentors
  ([788](https://github.com/open-telemetry/opentelemetry-python/pull/788))
- `opentelemetry-ext-grpc` Add metric recording (bytes in/out, errors, latency) to gRPC client

### Changed
- `opentelemetry-ext-pyramid` Use one general exclude list instead of two
  ([#872](https://github.com/open-telemetry/opentelemetry-python/pull/872))
- `opentelemetry-ext-boto` fails to export spans via jaeger
  ([#866](https://github.com/open-telemetry/opentelemetry-python/pull/866))
- `opentelemetry-ext-botocore` fails to export spans via jaeger
  ([#866](https://github.com/open-telemetry/opentelemetry-python/pull/866))
- `opentelemetry-ext-wsgi` Set span status on wsgi errors
  ([#864](https://github.com/open-telemetry/opentelemetry-python/pull/864))
- `opentelemetry-ext-flask` Use one general exclude list instead of two
  ([#872](https://github.com/open-telemetry/opentelemetry-python/pull/872))
- `opentelemetry-ext-django` Use one general exclude list instead of two
  ([#872](https://github.com/open-telemetry/opentelemetry-python/pull/872))
- `opentelemetry-ext-asyncpg` Shouldn't capture query parameters by default
  ([#854](https://github.com/open-telemetry/opentelemetry-python/pull/854))
- `opentelemetry-ext-mysql` bugfix: Fix auto-instrumentation entry point for mysql
  ([#858](https://github.com/open-telemetry/opentelemetry-python/pull/858))

## [0.10b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.10.0) - 2020-06-23

### Added
- `opentelemetry-ext-pymemcache` Initial release
- `opentelemetry-ext-elasticsearch` Initial release
- `opentelemetry-ext-celery` Add instrumentation for Celery
  ([#780](https://github.com/open-telemetry/opentelemetry-python/pull/780))
- `opentelemetry-instrumentation-starlette` Initial release
  ([#777](https://github.com/open-telemetry/opentelemetry-python/pull/777))
- `opentelemetry-ext-asyncpg` Initial Release
  ([#814](https://github.com/open-telemetry/opentelemetry-python/pull/814))

## [0.9b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.9.0) - 2020-06-10

### Added
- `opentelemetry-ext-pyramid` Initial release
- `opentelemetry-ext-boto` Initial release
- `opentelemetry-ext-botocore` Initial release
- `opentelemetry-ext-system-metrics` Initial release
  (https://github.com/open-telemetry/opentelemetry-python/pull/652)

## [0.8b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.8.0) - 2020-05-27

### Added
- `opentelemetry-ext-datadog` Add exporter to Datadog
  ([#572](https://github.com/open-telemetry/opentelemetry-python/pull/572))
- `opentelemetry-ext-sqlite3` Initial release
- `opentelemetry-ext-psycopg2` Implement instrumentor interface, enabling auto-instrumentation
  ([#694]https://github.com/open-telemetry/opentelemetry-python/pull/694)
- `opentelemetry-ext-asgi` Add ASGI middleware
  ([#716](https://github.com/open-telemetry/opentelemetry-python/pull/716))
- `opentelemetry-ext-django` Add exclude list for paths and hosts to prevent from tracing
  ([#670](https://github.com/open-telemetry/opentelemetry-python/pull/670))
- `opentelemetry-ext-django` Add support for django >= 1.10 (#717)

### Changed
- `opentelemetry-ext-grpc` lint: version of grpc causes lint issues
  ([#696](https://github.com/open-telemetry/opentelemetry-python/pull/696))

## [0.7b1](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.7.1) - 2020-05-12

### Added
- `opentelemetry-ext-redis` Initial release
- `opentelemetry-ext-jinja2` Add jinja2 instrumentation
  ([#643](https://github.com/open-telemetry/opentelemetry-python/pull/643))
- `opentelemetry-ext-pymongo` Implement instrumentor interface
  ([#612](https://github.com/open-telemetry/opentelemetry-python/pull/612))
- `opentelemetry-ext-sqlalchemy` Initial release
- `opentelemetry-ext-aiohttp-client` Initial release
- `opentelemetry-ext-pymysql` Initial release
- `opentelemetry-ext-http-requests` Implement instrumentor interface, enabling auto-instrumentation
  ([#597](https://github.com/open-telemetry/opentelemetry-python/pull/597))
- `opentelemetry-ext-http-requests` Adding disable_session for more granular instrumentation control
  ([#573](https://github.com/open-telemetry/opentelemetry-python/pull/573))
- `opentelemetry-ext-http-requests` Add a callback for custom attributes
  ([#656](https://github.com/open-telemetry/opentelemetry-python/pull/656))
- `opentelemetry-ext-dbapi` Implement instrument_connection and uninstrument_connection
  ([#624](https://github.com/open-telemetry/opentelemetry-python/pull/624))
- `opentelemetry-ext-flask` Add exclude list for paths and hosts
  ([#630](https://github.com/open-telemetry/opentelemetry-python/pull/630))
- `opentelemetry-ext-django` Initial release
- `opentelemetry-ext-mysql` Implement instrumentor interface
  ([#654](https://github.com/open-telemetry/opentelemetry-python/pull/654))

### Changed
- `opentelemetry-ext-http-requests` Rename package to opentelemetry-ext-requests
  ([#619](https://github.com/open-telemetry/opentelemetry-python/pull/619))

## [0.6b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.6.0) - 2020-03-30

### Added
- `opentelemetry-ext-flask` Add an entry_point to be usable in auto-instrumentation
  ([#327](https://github.com/open-telemetry/opentelemetry-python/pull/327))
- `opentelemetry-ext-grpc` Add gRPC integration
  ([#476](https://github.com/open-telemetry/opentelemetry-python/pull/476))

## [0.5b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.5.0) - 2020-03-16

## [0.4a0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.4.0) - 2020-02-21

### Added
- `opentelemetry-ext-psycopg2` Initial release
- `opentelemetry-ext-dbapi` Initial release
- `opentelemetry-ext-mysql` Initial release

### Changed
- `opentelemetry-ext-pymongo` Updating network connection attribute names
  ([#350](https://github.com/open-telemetry/opentelemetry-python/pull/350))
- `opentelemetry-ext-wsgi` Updating network connection attribute names
  ([#350](https://github.com/open-telemetry/opentelemetry-python/pull/350))
- `opentelemetry-ext-flask` Use string keys for WSGI environ values
  ([#366](https://github.com/open-telemetry/opentelemetry-python/pull/366))

## [0.3a0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.3.0) - 2019-12-11

### Added
- `opentelemetry-ext-flask` Initial release
- `opentelemetry-ext-pymongo` Initial release

### Changed
- `opentelemetry-ext-wsgi` Support new semantic conventions
  ([#299](https://github.com/open-telemetry/opentelemetry-python/pull/299))
- `opentelemetry-ext-wsgi` Updates for core library changes

## [0.2a0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.2.0) - 2019-10-29

### Changed
- `opentelemetry-ext-wsgi` Updates for core library changes
- `opentelemetry-ext-http-requests` Updates for core library changes

## [0.1a0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v0.1.0) - 2019-09-30

### Added
- `opentelemetry-ext-wsgi` Initial release
- `opentelemetry-ext-http-requests` Initial release
