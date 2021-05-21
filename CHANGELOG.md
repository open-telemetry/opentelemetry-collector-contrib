# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.18.0] - 2020-05-11

### Added
- `file_input` now supports multi-level directory globs (i.e. `/var/**/logs/**/*.log`) ([PR97](https://github.com/open-telemetry/opentelemetry-log-collection/pull/97))
- `add`, `remove`, `move`, `copy`, `retain`, and `flatten` operators, as alternatives to the `restructure` operator. 
- `add_attributes` option to `tcp_input` and `udp_input`, for capturing network attributes ([PR108](https://github.com/open-telemetry/opentelemetry-log-collection/pull/108))
- `multiline`, `encoding`, and `max_log_size` options to `tcp_input` ([PR125](https://github.com/open-telemetry/opentelemetry-log-collection/pull/125))
## Unreleased

### Added
- GoSec workflow added to GitHub Actions ([PR154](https://github.com/open-telemetry/opentelemetry-log-collection/pull/154))
- CodeQL workflow added to GitHub Actions ([PR153](https://github.com/open-telemetry/opentelemetry-log-collection/pull/153))

### Removed
- Database package. The same functionality is supported via a `Persister` interface, passed to `Start` methods ([PR93](https://github.com/open-telemetry/opentelemetry-log-collection/pull/93))

### Fixed
- Issue where `tcp_input` could panic or spam logs ([PR130](https://github.com/open-telemetry/opentelemetry-log-collection/pull/130))

## [0.17.0] - 2020-04-07

### Added
- Trace fields added to `entry.Entry`, and an accompanying `trace` parser ([PR76](https://github.com/open-telemetry/opentelemetry-log-collection/pull/76))
- Severity parser can parse whole numbers formatted as `float64` ([PR90](https://github.com/open-telemetry/opentelemetry-log-collection/pull/90))
- Support for `mapstructure` to most configs 

### Changed
- Rename `entry.Record` to `entry.Body` ([PR88](https://github.com/open-telemetry/opentelemetry-log-collection/pull/88))
## [0.16.0] - 2020-03-08

### Changed
- `syslog_input` config embeds `syslog_parser` at the top level, rather than under a `syslog` key ([PR43](https://github.com/open-telemetry/opentelemetry-log-collection/pull/43))
- Rename `entry.Label` to `entry.Attribute` ([PR51](https://github.com/open-telemetry/opentelemetry-log-collection/pull/51))

### Removed
- Several unused packages, including `flusher`, `buffer`, `k8smetadata`, `hostmetadata`, and `ratelimit` ([PR53](https://github.com/open-telemetry/opentelemetry-log-collection/pull/53))

## [0.15.1] - 2020-03-01

### Added
- Optional `max_buffer_size` parameter to `tcp_input` operator ([PR35](https://github.com/open-telemetry/opentelemetry-log-collection/pull/35))
- TLS support to `tcp_input` operator ([PR29](https://github.com/open-telemetry/opentelemetry-log-collection/pull/29))

### Fixed 
- Data race in syslog parser ([PR32](https://github.com/open-telemetry/opentelemetry-log-collection/pull/32))


## [0.15.0] - 2020-02-25

### Added
- `syslog_input` operator, which combines `tcp_input`, `udp_input`, and `syslog_parser` into a single operator. ([PR24](https://github.com/open-telemetry/opentelemetry-log-collection/pull/24))
- Syslog operator RFC 3164 location parameter ([PR11](https://github.com/open-telemetry/opentelemetry-log-collection/pull/11))
- `uri_parser` operator ([PR12](https://github.com/open-telemetry/opentelemetry-log-collection/pull/12))

### Removed
- `forward_input` and `forward_output`, which were previously intended for use in the standalone agent ([PR27](https://github.com/open-telemetry/opentelemetry-log-collection/pull/27))

## [0.14.0] - 2020-02-02

### Changed
- Remove standalone agent functionality
- Simplifies modules
  - Combines `parser/syslog` and `input/windows` modules into the primary module
  - Removes output operators that were previously separate modules
  - Leaves `input/k8sevent` and `transformer/k8smetadata` as separate modules for now. These two have extensive dependencies and their usefulness in the collector needs to be discussed before merging or removing.

## [0.13.12] - 2020-01-26

### Changed
- Allow plugin parameters to have a default value even if they are required

## [0.13.11] - 2020-01-15

### Changed
- Updated version of stanza used in several isolated modules

## [0.13.10] - 2020-01-15

### Added
- `timestamp` parser now supports a `location` parameter

## [0.13.9] - 2020-01-04

### Fixed
- `k8s_metadata_decorator` using a proxy causes internal API timeout

## [0.13.8] - 2020-12-30
### Fixed
- `file_input` exclude processing could result in extra exclusions

## [0.13.7] - 2020-12-23
### Added
- Ability to customize `file_input`'s `fingerprint_size`
## [0.13.6] - 2020-12-18
### Fixed
- Issue where timestamps ending 'Z' were not treated as UTC
- Issue where recognized timezones may not properly calculate offsets
- Issue where `file_output` would escape html special characters

## [0.13.5] - 2020-12-09
### Fixed
- Issue where flushers would retry indefinitely
- Issue where flushers would improperly reuse the same http request multiple times

## [0.13.4] - 2020-12-07
### Added
- Recombine operator to combine multiline logs after ingestion and parsing

### Fixed
- Issue where entries skipped by `if` would be output twice

## [0.13.3] - 2020-12-01
### Added
- New operators `forward_output` and `forward_input` to easily send log entries between stanza instances.
- Override default timestamp with `STANZA_DEFAULT_TIMESTAMP` for integration testing
- Add new `bytesize` type for easier configuration of byte sizes
- Automatic severity promotion in the syslog parser
### Fixed
- Open files in chunks so that we don't hit open file limit and cause performance issues

## [0.13.2] - 2020-11-17
### Added
- New parameter `if` to parser plugins to allow for easy conditional parsing without routers
- New `default` parameter to the router to explicitly send unmatched entries to a specific operator(s)

## [0.13.1] - 2020-11-11
### Fixed
- Missing default configuration of `elastic_output` flusher
### Changed
- A plugin that fails to parse will now log an error, but will not cause stanza to fail to start
### Added
- New `stdin` operator

## [0.13.0] - 2020-11-09
### Added
- OTLP severity level recognition
- Severity Text field on Entry
### Changed
- Removed `preserve` in favor of `preserve_to` to make it more clear that it may overwrite parsed fields
- Updated our internal log sampling numbers to more aggressively sample repeated logs
### Added
- Log message whenever a new file is detected

## [0.12.5] - 2020-10-07
### Added
- `windows_eventlog_input` can now parse messages from the Security channel.

## [0.12.4] - 2020-10-07
### Fixed
- Router outputs were not namespaced correctly

## [0.12.3] - 2020-10-07
### Fixed
- (De)serialization of JSON for plugin config structs

## [0.12.2] - 2020-10-06
### Added
- New Relic Logs output operator
- Additional resource values with parent object names (service name, replica set name, etc.) in the k8s metadata operator
- Publicly available `version.GetVersion()` for consumers of the `stanza` module

## [0.12.0] - 2020-09-21
### Changed
- Most operators are no longer part of dedicated modules

## [0.11.0] - 2020-09-15
### Changed
- File input improvements and rotation tests

## [0.10.0] - 2020-09-11
### Added
- Disk buffer for output operators ([PR109](https://github.com/observIQ/stanza/pull/109))
### Changed
- Split buffers into buffers and flushers for better modularity ([PR109](https://github.com/observIQ/stanza/pull/109))
- New memory buffer design for a uniform interface between disk and memory buffers ([PR109](https://github.com/observIQ/stanza/pull/109))
- Most operators are now dedicated modules, so that they may be imported individually ([PR108](https://github.com/observIQ/stanza/pull/108))

## [0.9.14] - 2020-08-31
### Fixed
- Rendering issue with the `kubernetes_events` plugin

## [0.9.13] - 2020-08-31
### Added
- Support for accessing the resource with fields ([PR105](https://github.com/observIQ/stanza/pull/105))
- Support for using fields to select keys that contain dots like `$record['field.with.dots']` ([PR105](https://github.com/observIQ/stanza/pull/105))
- `google_cloud_output` will use resource create a monitored resource for supported resource types (currently only k8s resources) ([PR105](https://github.com/observIQ/stanza/pull/105))
### Changed
- The operators `host_metadata`, `k8s_event_input`, and `k8s_metadata_decorator` will now use the top-level resource field ([PR105](https://github.com/observIQ/stanza/pull/105))
- `k8s_metadata_decorator` now generates pod labels that match those generated by GKE ([PR105](https://github.com/observIQ/stanza/pull/105))
### Fixed
- Issue with `k8s_event_input` generating entries with zero-valued time ([PR105](https://github.com/observIQ/stanza/pull/105))
- Plugin ID in templates will now correctly default to the plugin type if unset ([PR105](https://github.com/observIQ/stanza/pull/105))


## [0.9.12] - 2020-08-25
### Changed
- Agent is now embeddable with a default output

## [0.9.11] - 2020-08-24
### Added
- The 'filter' operator

### Changed
- Renamed project to `stanza`
- Move `testutil` package out of `internal`

## [0.9.10] - 2020-08-20
### Added
- The `Resource` field was added to Entry ([PR95](https://github.com/observIQ/stanza/pull/95))
- The `Identifier` helper was created to assist with writing to `Resource` ([PR95](https://github.com/observIQ/stanza/pull/95))

### Removed
- The `Tags` field was removed from Entry ([PR95](https://github.com/observIQ/stanza/pull/95))

### Changed
- The `host_metadata` operator now writes to an entry's `Resource` field, instead of Labels
- The `host_labeler` helper has been renamed `host_identifier`
- The `metadata` operator embeds the `Identifier` helper and supports writing to `Resource`
- Input operators embed the `Identifier` helper and support writing to `Resource`
- The `k8s_event` operator now supports the `write_to`, `labels`, and `resource` configuration options
- Multiline for `file_input` now supports matching on new lines characters ([PR96](https://github.com/observIQ/stanza/pull/96))

## [0.9.9] - 2020-08-14
### Added
- Kubernetes events input operator ([PR88](https://github.com/observIQ/stanza/pull/88))
### Fixed
- Small improvements to test stability
- Fallback to reflection to convert entries to Google Cloud log entries ([PR93](https://github.com/observIQ/stanza/pull/93))

## [0.9.8] - 2020-08-12
### Fixed
- Google Cloud Output failure when sent a field of type uint16 ([PR82](https://github.com/observIQ/stanza/pull/82))
### Added
- Added a default function to plugin templates ([PR84](https://github.com/observIQ/stanza/pull/84))
- Add a host metadata operator that adds hostname and IP to entries ([PR85](https://github.com/observIQ/stanza/pull/85))
- Google Cloud Output option to enable gzip compression ([PR86](https://github.com/observIQ/stanza/pull/86))

## [0.9.7] - 2020-08-05
### Changed
- In the file input operator, file name and path fields are now added with `include_file_name` (default `true`) and `include_file_path` (default `false`)
- Input and router operators can define labels on entries using the `labels` field
- Add Event ID to windows event log entries
- Use the `go-syslog` fork directly rather than relying on a `replace` directive so that the agent can be used as a library successfully

## [0.9.6] - 2020-08-04
### Changed
- Fork go-syslog to support long sdnames that are not rfc5424-compliant
- Reduce noise in debug messages for TCP and UDP inputs
### Added
- `log_type` label added by default to input operators
### Fixed
- Trim carriage returns from TCP input

## [0.9.5] - 2020-07-28
### Added
- Configurable `timeout` parameter for the `k8s_metadata_decorator` ([PR54](https://github.com/observIQ/stanza/pull/54))
- Journald operator now supports `start_at` parameter ([PR55](https://github.com/observIQ/stanza/pull/55))

### Changed
- Enhanced plugin parameter metadata structure, to support required/optional and default values ([PR59](https://github.com/observIQ/stanza/pull/59))

### Fixed
- Issue where multiple instances of `syslog_parser` would cause parsing errors ([PR61](https://github.com/observIQ/stanza/pull/61))
- `short destination buffer` error now is handled by increasing encoding buffer size ([PR58](https://github.com/observIQ/stanza/pull/58))
- Issue where omitting the output field in a plugin could result in errors ([PR56](https://github.com/observIQ/stanza/pull/56))

## [0.9.4] - 2020-07-21
- Allow omitting `id`, defaulting to plugin type if unique within namespace
- Allow omitting `output`, defaulting to the next operator in the pipeline if valid

## [0.9.3] - 2020-07-20
### Added
- Support for multiple encodings in the file input plugin ([PR39](https://github.com/observIQ/stanza/pull/39))
- Install scripts and docker image now include plugins from `stanza-plugins` repository ([PR45](https://github.com/observIQ/stanza/pull/45))
- Publish image to dockerhub ([PR42](https://github.com/observIQ/stanza/pull/42))
- Improved default configuration ([PR41](https://github.com/observIQ/stanza/pull/41))
- Basic developer documentation ([PR43](https://github.com/observIQ/stanza/pull/43))
### Fixed
- JournalD emits `map[string]interface{}` ([PR38](https://github.com/observIQ/stanza/pull/38))

## [0.9.2] - 2020-07-13
### Added
- Link `stanza` into `/usr/local/bin` so it's available on most users' `PATH` ([PR28](https://github.com/observIQ/stanza/pull/28))
- New parameter `file_name_path` to the file input plugin for cases when just the file name is needed
### Changed
- Renamed `path_field` to `file_path_field` in the file input plugin
### Fixed
- Failure in Google Cloud Output to convert some data types to protocol buffers

## [0.9.1] - 2020-07-13
### Added
- More specific warning and error messages for common configuration issues ([PR12](https://github.com/observIQ/stanza/pull/12),[PR13](https://github.com/observIQ/stanza/pull/13),[PR14](https://github.com/observIQ/stanza/pull/14))
### Fixed
- Writing from files being actively written to will sometimes read partial entries ([PR21](https://github.com/observIQ/stanza/pull/21))
- Minor documentation omissions

## [0.9.0] - 2020-07-07
### Added
- Initial open source release. See documentation for full list of supported features in this version.
