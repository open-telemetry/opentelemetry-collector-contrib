# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/open-telemetry/opentelemetry-python/compare/v1.12.0rc1-0.31b0...HEAD)

## [1.12.0rc1-0.31b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.12.0rc1-0.31b0) - 2022-05-17



### Fixed
- `opentelemetry-instrumentation-aiohttp-client` make span attributes available to sampler
  ([1072](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1072))
- `opentelemetry-instrumentation-aws-lambda` Fixed an issue - in some rare cases (API GW proxy integration test)
  headers are set to None, breaking context propagators.
  ([#1055](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1055))
- Refactoring custom header collection API for consistency
  ([#1064](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1064))
- `opentelemetry-instrumentation-sqlalchemy` will correctly report `otel.library.name`
  ([#1086](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1086))

### Added
- `opentelemetry-instrument` and `opentelemetry-bootstrap` now include a `--version` flag
  ([#1065](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1065))
- `opentelemetry-instrumentation-redis` now instruments asynchronous Redis clients, if the installed redis-py includes async support (>=4.2.0).
  ([#1076](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1076))

## [1.11.1-0.30b1](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.11.1-0.30b1) - 2022-04-21

### Added
- `opentelemetry-instrumentation-starlette` Capture custom request/response headers in span attributes
  ([#1046])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1046)

### Fixed
- Prune autoinstrumentation sitecustomize module directory from PYTHONPATH immediately
  ([#1066](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1066))

## [1.11.0-0.30b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.11.0-0.30b0) - 2022-04-18

### Fixed
- `opentelemetry-instrumentation-pyramid` Fixed which package is the correct caller in _traced_init.
  ([#830](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/830))
- `opentelemetry-instrumentation-tornado` Fix Tornado errors mapping to 500
  ([#1048](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1048))
- `opentelemetry-instrumentation-urllib` make span attributes available to sampler
  ([1014](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1014))
- `opentelemetry-instrumentation-flask` Fix non-recording span bug
  ([#999](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/999))
- `opentelemetry-instrumentation-tornado` Fix non-recording span bug
  ([#999](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/999))

### Added

- `opentelemetry-instrumentation-fastapi` Capture custom request/response headers in span attributes
  ([#1032])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1032)
- `opentelemetry-instrumentation-django` Capture custom request/response headers in span attributes
  ([#1024])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1024)
- `opentelemetry-instrumentation-asgi` Capture custom request/response headers in span attributes
  ([#1004])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1004)
- `opentelemetry-instrumentation-psycopg2` extended the sql commenter support of dbapi into psycopg2
  ([#940](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/940))
- `opentelemetry-instrumentation-falcon` Add support for falcon==1.4.1
  ([#1000])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1000)
- `opentelemetry-instrumentation-falcon` Falcon: Capture custom request/response headers in span attributes
  ([#1003])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1003)
- `opentelemetry-instrumentation-elasticsearch` no longer creates unique span names by including search target, replaces them with `<target>` and puts the value in attribute `elasticsearch.target`
  ([#1018](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1018))
- `opentelemetry-instrumentation-pyramid` Handle non-HTTPException exceptions
  ([#1001](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1001))
- `opentelemetry-instrumentation-system-metrics` restore `SystemMetrics` instrumentation as `SystemMetricsInstrumentor`
  ([#1012](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1012))
- `opentelemetry-instrumentation-pyramid` Pyramid: Capture custom request/response headers in span attributes
  ([#1022])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/1022)


## [1.10.0-0.29b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.10.0-0.29b0) - 2022-03-10

- `opentelemetry-instrumentation-wsgi` Capture custom request/response headers in span attributes
  ([#925])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/925)
- `opentelemetry-instrumentation-flask` Flask: Capture custom request/response headers in span attributes
  ([#952])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/952)
- `opentelemetry-instrumentation-tornado` Tornado: Capture custom request/response headers in span attributes
  ([#950])(https://github.com/open-telemetry/opentelemetry-python-contrib/pull/950)

### Added

- `opentelemetry-instrumentation-aws-lambda` `SpanKind.SERVER` by default, add more cases for `SpanKind.CONSUMER` services. ([#926](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/926))
- `opentelemetry-instrumentation-sqlalchemy` added experimental sql commenter capability
   ([#924](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/924))
- `opentelemetry-contrib-instrumentations` added new meta-package that installs all contrib instrumentations.
  ([#681](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/681))
- `opentelemetry-instrumentation-dbapi` add experimental sql commenter capability
  ([#908](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/908))
- `opentelemetry-instrumentation-requests` make span attribute available to samplers
  ([#931](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/931))
- `opentelemetry-datadog-exporter` add deprecation note to example.
  ([#900](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/900))

### Fixed

- `opentelemetry-instrumentation-dbapi` Changed the format of traceparent id.
  ([#941](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/941))
- `opentelemetry-instrumentation-logging` retrieves service name defensively.
  ([#890](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/890))
- `opentelemetry-instrumentation-wsgi` WSGI: Conditionally create SERVER spans
  ([#903](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/903))
- `opentelemetry-instrumentation-falcon` Safer patching mechanism
  ([#895](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/895))
- `opentelemetry-instrumentation-kafka-python` Fix topic extraction
  ([#949](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/949))

### Changed

- `opentelemetry-instrumentation-pymemcache` should run against newer versions of pymemcache.
  ([#935](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/935))

## [1.9.1-0.28b1](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.9.1-0.28b1) - 2022-01-29

### Fixed

- `opentelemetry-instrumentation-pika` requires `packaging` dependency

- `opentelemetry-instrumentation-tornado` Tornado: Conditionally create SERVER spans
  ([#889](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/889))

## [1.9.0-0.28b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.9.0-0.28b0) - 2022-01-26


### Added

- `opentelemetry-instrumentation-pyramid` Pyramid: Conditionally create SERVER spans
  ([#869](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/869))
- `opentelemetry-instrumentation-grpc` added `trailing_metadata` to _OpenTelemetryServicerContext.
  ([#871](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/871))
- `opentelemetry-instrumentation-asgi` now returns a `traceresponse` response header.
  ([#817](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/817))
- `opentelemetry-instrumentation-kafka-python` added kafka-python module instrumentation.
  ([#814](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/814))
- `opentelemetry-instrumentation-falcon` Falcon: Conditionally create SERVER spans
  ([#867](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/867))
- `opentelemetry-instrumentation-pymongo` now supports `pymongo v4`
  ([#876](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/876))

- `opentelemetry-instrumentation-httpx` now supports versions higher than `0.19.0`.
  ([#866](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/866))

### Fixed

- `opentelemetry-instrumentation-django` Django: Conditionally create SERVER spans
  ([#832](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/832))
- `opentelemetry-instrumentation-flask` Flask: Conditionally create SERVER spans
  ([#828](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/828))
- `opentelemetry-instrumentation-celery` Celery: Support partial task time limit
  ([#846](https://github.com/open-telemetry/opentelemetry-python-contrib/issues/846))
- `opentelemetry-instrumentation-asgi` ASGI: Conditionally create SERVER spans
  ([#843](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/843))
- `opentelemetry-instrumentation-django` Django: fix issue preventing detection of MIDDLEWARE_CLASSES
- `opentelemetry-instrumentation-sqlite3` Instrumentation now works with `dbapi2.connect`
- `opentelemetry-instrumentation-kafka` Kafka: safe kafka partition extraction
  ([#872](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/872))
- `opentelemetry-instrumentation-aiohttp-client` aiohttp: Correct url filter input type
  ([#843](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/864))

- `opentelemetry-instrumentation-aiohttp-client` aiohttp: Remove `span_name` from docs
  ([#857](https://github.com/open-telemetry/opentelemetry-python-contrib/issues/857))


## [1.8.0-0.27b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.8.0-0.27b0) - 2021-12-17

### Added

- `opentelemetry-instrumentation-aws-lambda` Adds support for configurable flush timeout  via `OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT` property. ([#825](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/825))
- `opentelemetry-instrumentation-pika` Adds support for versions between `0.12.0` to `1.0.0`. ([#837](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/837))

### Fixed

- `opentelemetry-instrumentation-urllib` Fixed an error on unexpected status values.
  ([#823](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/823))

- `opentelemetry-exporter-richconsole` Fixed attribute error on parentless spans.
  ([#782](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/782))

- `opentelemetry-instrumentation-tornado` Add support instrumentation for Tornado 5.1.1
  ([#812](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/812))

## [1.7.1-0.26b1](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.7.0-0.26b0) - 2021-11-11

### Added

- `opentelemetry-instrumentation-aws-lambda` Add instrumentation for AWS Lambda Service - pkg metadata files (Part 1/2)
  ([#739](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/739))
- Add support for Python 3.10
  ([#742](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/742))
- Pass in auto-instrumentation version to configurator
  ([#783](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/783))
- `opentelemetry-instrumentation` Add `setuptools` to `install_requires`
  ([#781](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/781))
- `opentelemetry-instrumentation-aws-lambda` Add instrumentation for AWS Lambda Service - Implementation (Part 2/2)
  ([#777](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/777))
- `opentelemetry-instrumentation-pymongo` Add `request_hook`, `response_hook` and `failed_hook` callbacks passed as arguments to the instrument method
  ([#793](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/793))
- `opentelemetry-instrumentation-pymysql` Add support for PyMySQL 1.x series
  ([#792](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/792))
- Add support for generic OTEL_PYTHON_EXCLUDED_URLS variable
  ([#790](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/790))

### Fixed

- `opentelemetry-instrumentation-asgi` now explicitly depends on asgiref as it uses the package instead of instrumenting it.
  ([#765](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/765))
- `opentelemetry-instrumentation-pika` now propagates context to basic_consume callback
  ([#766](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/766))
- `opentelemetry-instrumentation-falcon` Dropped broken support for Python 3.4.
  ([#774](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/774))
- `opentelemetry-instrumentation-django` Fixed carrier usage on ASGI requests.
  ([#767](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/767))
- Don't set Span Status on 4xx http status code for SpanKind.SERVER spans
  ([#776](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/776))
- `opentelemetry-instrumentation-django` Fixed instrumentation and tests for all Django major versions.
  ([#780](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/780))

## [1.6.2-0.25b2](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.6.2-0.25b2) - 2021-10-19

- `opentelemetry-instrumentation-sqlalchemy` Fix PostgreSQL instrumentation for Unix sockets
  ([#761](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/761))

### Changed

- `opentelemetry-sdk-extension-aws` & `opentelemetry-propagator-aws` Release AWS Python SDK Extension as 2.0.1 and AWS Propagator as 1.0.1
  ([#753](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/753))
- `opentelemetry-instrumentation-pika` Add `_decorate_basic_consume` to ensure post instrumentation `basic_consume` calls are also instrumented.
  ([#759](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/759))
- Consolidate instrumentation documentation in docstrings
  ([#754](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/754))

### Fixed

- `opentelemetry-distro` uses the correct entrypoint name which was updated in the core release of 1.6.0 but the distro was not updated with it
  ([#755](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/755))

### Added
- `opentelemetry-instrumentation-pika` Add `publish_hook` and `consume_hook` callbacks passed as arguments to the instrument method
  ([#763](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/763))


## [1.6.1-0.25b1](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.6.1-0.25b1) - 2021-10-18

### Changed
- `opentelemetry-util-http` no longer contains an instrumentation entrypoint and will not be loaded
  automatically by the auto instrumentor.
  ([#745](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/745))
- `opentelemetry-instrumentation-pika` Bugfix use properties.headers. It will prevent the header injection from raising.
  ([#740](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/740))
- `opentelemetry-instrumentation-botocore` Add extension for DynamoDB
  ([#735](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/735))
- `opentelemetry-sdk-extension-aws` & `opentelemetry-propagator-aws` Remove unnecessary dependencies on `opentelemetry-test`
  ([#752](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/752))
- `opentelemetry-instrumentation-botocore` Add Lambda extension
  ([#760](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/760))

## [1.6.0-0.25b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.6.0-0.25b0) - 2021-10-13
### Added
- `opentelemetry-sdk-extension-aws` Release AWS Python SDK Extension as 1.0.0
  ([#667](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/667))
- `opentelemetry-instrumentation-urllib3`, `opentelemetry-instrumentation-requests`
  The `net.peer.ip` attribute is set to the IP of the connected HTTP server or proxy
  using a new instrumentor in `opententelemetry-util-http`
  ([#661](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/661))
- `opentelemetry-instrumentation-pymongo` Add check for suppression key in PyMongo.
  ([#736](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/736))
- `opentelemetry-instrumentation-elasticsearch` Added `response_hook` and `request_hook` callbacks
  ([#670](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/670))
- `opentelemetry-instrumentation-redis` added request_hook and response_hook callbacks passed as arguments to the instrument method.
  ([#669](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/669))
- `opentelemetry-instrumentation-botocore` add `request_hook` and `response_hook` callbacks
  ([679](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/679))
- `opentelemetry-exporter-richconsole` Initial release
  ([#686](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/686))
- `opentelemetry-instrumentation-elasticsearch` no longer creates unique span names by including document IDs, replaces them with `:id` and puts the value in attribute `elasticsearch.id`
  ([#705](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/705))
- `opentelemetry-instrumentation-tornado` now sets `http.client_ip` and `tornado.handler` attributes
  ([#706](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/706))
- `opentelemetry-instrumentation-requests` added exclude urls functionality
  ([#714](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/714))
- `opentelemetry-instrumentation-django` Add ASGI support
  ([#391](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/391))

### Changed
- `opentelemetry-instrumentation-flask` Fix `RuntimeError: Working outside of request context`
  ([#734](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/734))
- `opentelemetry-propagators-aws-xray` Rename `AwsXRayFormat` to `AwsXRayPropagator`
  ([#729](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/729))
- `opentelemetry-instrumentation-sqlalchemy` Respect provided tracer provider when instrumenting SQLAlchemy
  ([#728](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/728))
- `opentelemetry-sdk-extension-aws` Move AWS X-Ray Propagator into its own `opentelemetry-propagators-aws` package
  ([#720](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/720))
- `opentelemetry-instrumentation-sqlalchemy` Added `packaging` dependency
  ([#713](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/713))
- `opentelemetry-instrumentation-jinja2` Allow instrumentation of newer Jinja2 versions.
  ([#712](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/712))
- `opentelemetry-instrumentation-botocore` Make common span attributes compliant with semantic conventions
  ([#674](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/674))
- `opentelemetry-sdk-extension-aws` Release AWS Python SDK Extension as 1.0.0
  ([#667](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/667))
- `opentelemetry-instrumentation-botocore` Unpatch botocore Endpoint.prepare_request on uninstrument
  ([#664](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/664))
- `opentelemetry-instrumentation-botocore` Fix span injection for lambda invoke
  ([#663](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/663))
- `opentelemetry-instrumentation-botocore` Introduce instrumentation extensions
  ([#718](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/718))
- `opentelemetry-instrumentation-urllib3` Updated `_RequestHookT` with two additional fields - the request body and the request headers
  ([#660](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/660))
- Tests for Falcon 3 support
  ([#644](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/644))

## [1.5.0-0.24b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.5.0-0.24b0) - 2021-08-26

### Added
- `opentelemetry-sdk-extension-aws` Add AWS resource detectors to extension package
  ([#586](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/586))
- `opentelemetry-instrumentation-asgi`, `opentelemetry-instrumentation-aiohttp-client`, `openetelemetry-instrumentation-fastapi`,
  `opentelemetry-instrumentation-starlette`, `opentelemetry-instrumentation-urllib`, `opentelemetry-instrumentation-urllib3` Added `request_hook` and `response_hook` callbacks
  ([#576](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/576))
- `opentelemetry-instrumentation-pika` added RabbitMQ's pika module instrumentation.
  ([#680](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/680))

### Changed

- `opentelemetry-instrumentation-fastapi` Allow instrumentation of newer FastAPI versions.
  ([#602](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/602))
- Enable explicit `excluded_urls` argument in `opentelemetry-instrumentation-flask`
  ([#604](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/604))

## [1.4.0-0.23b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.4.0-0.23b0) - 2021-07-21

### Removed
- Move `opentelemetry-instrumentation` to the core repo.
  ([#595](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/595))

### Changed
- `opentelemetry-instrumentation-falcon` added support for Falcon 3.
  ([#607](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/607))
- `opentelemetry-instrumentation-tornado` properly instrument work done in tornado on_finish method.
  ([#499](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/499))
- `opentelemetry-instrumentation` Fixed cases where trying to use an instrumentation package without the
  target library was crashing auto instrumentation agent.
  ([#530](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/530))
- Fix weak reference error for pyodbc cursor in SQLAlchemy instrumentation.
  ([#469](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/469))
- Implemented specification that HTTP span attributes must not contain username and password.
  ([#538](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/538))
- Changed the psycopg2-binary to psycopg2 as dependency in production
  ([#543](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/543))
- Implement consistent way of checking if instrumentation is already active
  ([#549](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/549))
- Require aiopg to be less than 1.3.0
  ([#560](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/560))
- `opentelemetry-instrumentation-django` Migrated Django middleware to new-style.
  ([#533](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/533))
- Updating dependency for opentelemetry api/sdk packages to support major version instead
  of pinning to specific versions.
  ([#567](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/567))
- `opentelemetry-instrumentation-grpc` Respect the suppress instrumentation in gRPC client instrumentor
  ([#559](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/559))
- `opentelemetry-instrumentation-grpc` Fixed asynchonous unary call traces
  ([#536](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/536))
- `opentelemetry-sdk-extension-aws` Update AWS entry points to match spec
  ([#566](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/566))
- Include Flask 2.0 as compatible with existing flask instrumentation
  ([#545](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/545))
- `openelemetry-sdk-extension-aws` Take a dependency on `opentelemetry-sdk`
  ([#558](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/558))
- Change `opentelemetry-instrumentation-httpx` to replace `client` classes with instrumented versions.
  ([#577](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/577))
- `opentelemetry-instrumentation-requests` Fix potential `AttributeError` when `requests`
  is used with a custom transport adapter.
  ([#562](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/562))
- `opentelemetry-instrumentation-django` Fix AttributeError: ResolverMatch object has no attribute route
  ([#581](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/581))
- `opentelemetry-instrumentation-botocore` Suppress botocore downstream instrumentation like urllib3
  ([#563](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/563))
- `opentelemetry-exporter-datadog` Datadog exporter should not use `unknown_service` as fallback resource service name.
  ([#570](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/570))
- Add support for the async extension of SQLAlchemy (>= 1.4)
  ([#568](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/568))

### Added
- `opentelemetry-instrumentation-httpx` Add `httpx` instrumentation
  ([#461](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/461))

## [0.22b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.3.0-0.22b0) - 2021-06-01

### Changed
- `opentelemetry-bootstrap` not longer forcibly removes and re-installs libraries and their instrumentations.
  This means running bootstrap will not auto-upgrade existing dependencies and as a result not cause dependency
  conflicts.
  ([#514](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/514))
- `opentelemetry-instrumentation-asgi` Set the response status code on the server span
  ([#478](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/478))
- `opentelemetry-instrumentation-tornado` Fixed cases where description was used with non-
  error status code when creating Status objects.
  ([#504](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/504))
- `opentelemetry-instrumentation-asgi` Fix instrumentation default span name.
  ([#418](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/418))
- Propagators use the root context as default for `extract` and do not modify
  the context if extracting from carrier does not work.
  ([#488](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/488))

### Added
- `opentelemetry-instrumentation-botocore` now supports
  context propagation for lambda invoke via Payload embedded headers.
  ([#458](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/458))
- Added support for CreateKey functionality.
  ([#502](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/502))

## [0.21b0](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v1.2.0-0.21b0) - 2021-05-11

### Changed
- Instrumentation packages don't specify the libraries they instrument as dependencies
  anymore. Instead, they verify the correct version of libraries are installed at runtime.
  ([#475](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/475))
- `opentelemetry-propagator-ot-trace` Use `TraceFlags` object in `extract`
  ([#472](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/472))
- Set the `traced_request_attrs` of FalconInstrumentor by an argument correctly.
  ([#473](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/473))
- Enable passing explicit urls to exclude in instrumentation in FastAPI
  ([#486](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/486))
- Distros can now implement `load_instrumentor(EntryPoint)` method to customize instrumentor
  loading behaviour.
  ([#480](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/480))
- Fix entrypoint for ottrace propagator
  ([#492](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/492))

### Added

- Move `opentelemetry-instrumentation` from core repository
  ([#465](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/465))

## [0.20b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.20b0) - 2021-04-20

### Changed

- Restrict DataDog exporter's `ddtrace` dependency to known working versions.
  ([#400](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/400))
- GRPC instrumentation now correctly injects trace context into outgoing requests.
  ([#392](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/39))
- Publish `opentelemetry-propagator-ot-trace` package as a part of the release process
  ([#387](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/387))
- Update redis instrumentation to follow semantic conventions
  ([#403](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/403))
- Update instrumentations to use tracer_provider for creating tracer if given, otherwise use global tracer provider
  ([#402](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/402))
- `opentelemetry-instrumentation-wsgi` Replaced `name_callback` with `request_hook`
  and `response_hook` callbacks.
  ([#424](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/424))
- Update gRPC instrumentation to better wrap server context
  ([#420](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/420))
- `opentelemetry-instrumentation-redis` Fix default port KeyError and Wrong Attribute name (net.peer.ip -> net.peer.port)
  ([#265](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/265))
- `opentelemetry-instrumentation-asyncpg` Fix default port KeyError and Wrong Attribute name (net.peer.ip -> net.peer.port)
  ([#265](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/265))

### Added

- `opentelemetry-instrumentation-urllib3` Add urllib3 instrumentation
  ([#299](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/299))

- `opentelemetry-instrumentation-flask` Added `request_hook` and `response_hook` callbacks.
  ([#416](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/416))

- `opentelemetry-instrumenation-django` now supports request and response hooks.
  ([#407](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/407))
- `opentelemetry-instrumentation-falcon` FalconInstrumentor now supports request/response hooks.
  ([#415](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/415))
- `opentelemetry-instrumentation-tornado` Add request/response hooks.
  ([#426](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/426))
- `opentelemetry-exporter-datadog` Add parsing exception events for error tags.
  ([#459](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/459))
- `opentelemetry-instrumenation-django` now supports trace response headers.
  ([#436](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/436))
- `opentelemetry-instrumenation-tornado` now supports trace response headers.
  ([#436](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/436))
- `opentelemetry-instrumenation-pyramid` now supports trace response headers.
  ([#436](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/436))
- `opentelemetry-instrumenation-falcon` now supports trace response headers.
  ([#436](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/436))
- `opentelemetry-instrumenation-flask` now supports trace response headers.
  ([#436](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/436))
- `opentelemetry-instrumentation-grpc` Keep client interceptor in sync with grpc client interceptors.
  ([#442](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/442))

### Removed

- Remove `http.status_text` from span attributes
  ([#406](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/406))

## [0.19b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.19b0) - 2021-03-26

- Implement context methods for `_InterceptorChannel`
  ([#363](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/363))

### Changed

- Rename `IdsGenerator` to `IdGenerator`
  ([#350](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/350))
- `opentelemetry-exporter-datadog` Fix warning when DatadogFormat encounters a request with
  no DD_ORIGIN headers ([#368](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/368)).
- `opentelemetry-instrumentation-aiopg` Fix multiple nested spans when
  `aiopg.pool` is used
  ([#336](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/381)).
- Updated instrumentations to use `opentelemetry.trace.use_span` instead of `Tracer.use_span()`
  ([#364](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/364))
- `opentelemetry-propagator-ot-trace` Do not throw an exception when headers are not present
  ([#378](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/378))
- `opentelemetry-instrumentation-wsgi` Reimplement `keys` method to return actual keys from the carrier instead of an empty list.
  ([#379](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/379))
- `opentelemetry-instrumentation-sqlalchemy` Fix multithreading issues in recording spans from SQLAlchemy
  ([#315](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/315))
- Make getters and setters optional
  ([#372](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/372))

### Removed

- Removing support for Python 3.5
  ([#374](https://github.com/open-telemetry/opentelemetry-python/pull/374))

## [0.18b0](https://github.com/open-telemetry/opentelemetry-python-contrib/releases/tag/v0.18b0) - 2021-02-16

### Added

- `opentelemetry-propagator-ot-trace` Add OT Trace Propagator
  ([#302](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/302))
- `opentelemetry-instrumentation-logging` Added logging instrumentation to enable log - trace correlation.
  ([#345](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/345))

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

- Fix broken links to project ([#413](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/413))
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
  ([#150](https://github.com/open-telemetry/opentelemetry-python-contrib/pull/150))
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
