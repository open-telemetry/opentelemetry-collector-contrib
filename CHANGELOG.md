# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.28.0] - 2022-03-17

### Changed
- Switch to original go-syslog library, restoring strict enforcement of SD-NAME length. ([PR439](https://github.com/open-telemetry/opentelemetry-log-collection/pull/439))

## [0.27.2] - 2022-03-17

### Fixed
- Revert version update on go-syslog, which introduced incompatibility with 386 architecture. ([PR438](https://github.com/open-telemetry/opentelemetry-log-collection/pull/438))

## [0.27.1] - 2022-03-16

### Fixed
- Issue where pipelines could fail to build when running on Go 1.18. ([PR347](https://github.com/open-telemetry/opentelemetry-log-collection/pull/347))

## [0.27.0] - 2022-03-10

### Added
- `csv_parser` can now handle fields containing line breaks. ([PR425]https://github.com/open-telemetry/opentelemetry-log-collection/pull/425)

### Fixed
- Issue where `recombine` operator would combine entire file in certain specific circumstances. ([PR416]https://github.com/open-telemetry/opentelemetry-log-collection/pull/416)


## [0.26.0] - 2022-02-25

### Added
- `source_identifier` setting to `recombine` operator, to ensure partial entries are joined to others from the same file, or other source. ([PR341](https://github.com/open-telemetry/opentelemetry-log-collection/pull/341))
- `max_sources` setting to `recombine` operator, which limits the number of unique sources that may accumulate partial entries. ([PR341](https://github.com/open-telemetry/opentelemetry-log-collection/pull/341))

### Fixed
- Time parsing will now correctly parse timestamps from 1970. ([PR417](https://github.com/open-telemetry/opentelemetry-log-collection/pull/417))
- Issue where `file_input` operator could duplicate a small number of log entries. ([PR413](https://github.com/open-telemetry/opentelemetry-log-collection/pull/413))

### Changed
- `entry.Attributes` data type from `map[string]string` to `map[string]interface{}`. ([PR401](https://github.com/open-telemetry/opentelemetry-log-collection/pull/401))
- `entry.Resource` data type from `map[string]string` to `map[string]interface{}`. ([PR411](https://github.com/open-telemetry/opentelemetry-log-collection/pull/411))

### Removed
- `write_to` configuration setting from all input operators. Use `move` operator instead. ([PR412](https://github.com/open-telemetry/opentelemetry-log-collection/pull/412))


## [0.25.0] - 2022-02-21

This release contains a few minor updates as well as a major cleanup of the codebase. See the [Reduce Complexity](https://github.com/open-telemetry/opentelemetry-log-collection/milestone/2?closed=1) milestone for full details on the cleanup.

### Added
- `source_identifier` setting to `recombine` operator, to ensure partial entries are joined to others from the same file, or other source. ([PR341](https://github.com/open-telemetry/opentelemetry-log-collection/pull/341))
- `max_sources` setting to `recombine` operator, which limits the number of unique sources that may accumulate partial entries. ([PR341](https://github.com/open-telemetry/opentelemetry-log-collection/pull/341))

### Fixed
- On Windows, `file_input` will immediately close files after reading. ([PR366](https://github.com/open-telemetry/opentelemetry-log-collection/pull/366))

### Changed
- When `file_input` cannot open a file, it will print a debug level log instead of an error level log. ([PR357](https://github.com/open-telemetry/opentelemetry-log-collection/pull/357))


## [0.24.0] - 2021-12-21

### Added
- `force_flush_period` setting to `recombine` operator, to prevent recombine taking to long to process ([PR325](https://github.com/open-telemetry/opentelemetry-log-collection/pull/325))
- `lazy_quotes` setting to `csv` parser operator. When enabled will preserve internal quotes in a csv field ([PR324](https://github.com/open-telemetry/opentelemetry-log-collection/pull/324))
- `header_attribute` setting to `csv` parser operator. When set will dynamically parse the csv headers from the specified attribute on a log entry. ([PR335](https://github.com/open-telemetry/opentelemetry-log-collection/pull/335))

### Changed
- Updated CSV Parser to use idiomatic Go errors ([PR323](https://github.com/open-telemetry/opentelemetry-log-collection/pull/323))

## [0.23.0] - 2021-11-30


### Added
- `combine_with` setting to `recombine` operator, to allow for joining on custom delimiter ([PR315](https://github.com/open-telemetry/opentelemetry-log-collection/pull/315))
### Fixed
- Issue where `force_flush_period` could cause line splitting to be skipped ([PR303](https://github.com/open-telemetry/opentelemetry-log-collection/pull/303))
- Issue where `tcp_input` and `udp_input` could panic when stopping ([PR273](https://github.com/open-telemetry/opentelemetry-log-collection/pull/273))
- Syslog severity mapping is now aligned with log specification ([PR300](https://github.com/open-telemetry/opentelemetry-log-collection/pull/300))

### Changed
- Improve error message when timezone database is not found ([PR289](https://github.com/open-telemetry/opentelemetry-log-collection/pull/289))



## [0.22.0] - 2021-10-05

### Fixed
- Issue in `file_input` where doublestar globbing could only be used at a single level ([PR268](https://github.com/open-telemetry/opentelemetry-log-collection/pull/268))
- Bug in `tcp_input`, `udp_input`, and `syslog_input` which could cause a panic ([PR273](https://github.com/open-telemetry/opentelemetry-log-collection/pull/273))
- Made `windows_event_log_input` compatibile with Windows Server 2022 ([PR283](https://github.com/open-telemetry/opentelemetry-log-collection/pull/283))

### Changed
- `file_input` will now emit bytes when `encoding = nop` ([PR262](https://github.com/open-telemetry/opentelemetry-log-collection/pull/262))

## [0.21.0] - 2021-09-09

### Added
- The `timestamp` operator can now parse timestamps that use a comma separator ([Go v1.17](https://golang.org/doc/go1.17#time))
- `journald_input` now accepts `units` and `priority` settings ([PR252](https://github.com/open-telemetry/opentelemetry-log-collection/pull/252))
- `file_input` will now trim whitespace when using `multiline` ([PR212](https://github.com/open-telemetry/opentelemetry-log-collection/pull/212))

### Changed
- Operator IDs are now autogenerated sequentially, removing the necessity to specify the `id` field in most circumstances ([PR246](https://github.com/open-telemetry/opentelemetry-log-collection/pull/246))
- Updated to go version 1.17 ([PR248](https://github.com/open-telemetry/opentelemetry-log-collection/pull/248))

### Fixed
- `file_input`'s `force_flush_period` now defaults to `500ms`, ensuring that the use of `multiline.line_start_regex` does not cause ommision of the last line of each file ([PR261](https://github.com/open-telemetry/opentelemetry-log-collection/pull/261))

## [0.20.0] - 2021-07-27

### Added
- `file_input` operator can now be configured to flush incomplete logs, using the `force_flush_period` setting ([PR216](https://github.com/open-telemetry/opentelemetry-log-collection/pull/216))

### Changed
- `severity` levels have been redefined to match OpenTelemetry standard levels ([PR228](https://github.com/open-telemetry/opentelemetry-log-collection/pull/228))
### Fixed
- `multiline` splitting now trims whitespace characters ([PR212](https://github.com/open-telemetry/opentelemetry-log-collection/pull/212))
- `windows_eventlog_input` log input now gives a helpful error message when a metadata request fails ([PR206](https://github.com/open-telemetry/opentelemetry-log-collection/pull/206))


## [0.19.0] - 2021-06-22

### Added
- `csv_parser` ([PR123](https://github.com/open-telemetry/opentelemetry-log-collection/pull/123))
- `multiline`, `encoding`, and `max_log_size` options to `udp_input` ([PR127](https://github.com/open-telemetry/opentelemetry-log-collection/pull/127))
- `file_input` now has `include_file_name_resolved` and `include_file_path_resolved` settings which produce attributes `file.name.resolved` and `file.path.resolved`, respectively ([PR189](https://github.com/open-telemetry/opentelemetry-log-collection/pull/189))
- GoSec workflow added to GitHub Actions ([PR154](https://github.com/open-telemetry/opentelemetry-log-collection/pull/154))
- CodeQL workflow added to GitHub Actions ([PR153](https://github.com/open-telemetry/opentelemetry-log-collection/pull/153))

### Changed
- `file_input`'s default `encoding` is now `utf8` ([PR147](https://github.com/open-telemetry/opentelemetry-log-collection/pull/147))
- `file_input`'s `include_file_name` and `include_file_path` settings now produce attributes `file.name` and `file.path`, respectively ([PR189](https://github.com/open-telemetry/opentelemetry-log-collection/pull/189))

### Fixed
- `file_input` can now track files that are rotated out of the `include` pattern matches ([PR182](https://github.com/open-telemetry/opentelemetry-log-collection/pull/182))
- Noisy log message in `file_input` ([PR174](https://github.com/open-telemetry/opentelemetry-log-collection/pull/174))
- Issue where failed parse operation could duplicate log entry ([PR188](https://github.com/open-telemetry/opentelemetry-log-collection/pull/188))

### Removed
- Parsers will no longer process `[]byte` data type ([PR149](https://github.com/open-telemetry/opentelemetry-log-collection/pull/149))

## [0.18.0] - 2021-05-11

### Added
- `file_input` now supports multi-level directory globs (i.e. `/var/**/logs/**/*.log`) ([PR97](https://github.com/open-telemetry/opentelemetry-log-collection/pull/97))
- `add`, `remove`, `move`, `copy`, `retain`, and `flatten` operators, as alternatives to the `restructure` operator. 
- `add_attributes` option to `tcp_input` and `udp_input`, for capturing network attributes ([PR108](https://github.com/open-telemetry/opentelemetry-log-collection/pull/108))
- `multiline`, `encoding`, and `max_log_size` options to `tcp_input` ([PR125](https://github.com/open-telemetry/opentelemetry-log-collection/pull/125))

### Removed
- Database package. The same functionality is supported via a `Persister` interface, passed to `Start` methods ([PR93](https://github.com/open-telemetry/opentelemetry-log-collection/pull/93))

### Fixed
- Issue where `tcp_input` could panic or spam logs ([PR130](https://github.com/open-telemetry/opentelemetry-log-collection/pull/130))

## [0.17.0] - 2021-04-07

### Added
- Trace fields added to `entry.Entry`, and an accompanying `trace` parser ([PR76](https://github.com/open-telemetry/opentelemetry-log-collection/pull/76))
- Severity parser can parse whole numbers formatted as `float64` ([PR90](https://github.com/open-telemetry/opentelemetry-log-collection/pull/90))
- Support for `mapstructure` to most configs 

### Changed
- Rename `entry.Record` to `entry.Body` ([PR88](https://github.com/open-telemetry/opentelemetry-log-collection/pull/88))
## [0.16.0] - 2021-03-08

### Changed
- `syslog_input` config embeds `syslog_parser` at the top level, rather than under a `syslog` key ([PR43](https://github.com/open-telemetry/opentelemetry-log-collection/pull/43))
- Rename `entry.Label` to `entry.Attribute` ([PR51](https://github.com/open-telemetry/opentelemetry-log-collection/pull/51))

### Removed
- Several unused packages, including `flusher`, `buffer`, `k8smetadata`, `hostmetadata`, and `ratelimit` ([PR53](https://github.com/open-telemetry/opentelemetry-log-collection/pull/53))

## [0.15.1] - 2021-03-01

### Added
- Optional `max_buffer_size` parameter to `tcp_input` operator ([PR35](https://github.com/open-telemetry/opentelemetry-log-collection/pull/35))
- TLS support to `tcp_input` operator ([PR29](https://github.com/open-telemetry/opentelemetry-log-collection/pull/29))

### Fixed 
- Data race in syslog parser ([PR32](https://github.com/open-telemetry/opentelemetry-log-collection/pull/32))


## [0.15.0] - 2021-02-25

### Added
- `syslog_input` operator, which combines `tcp_input`, `udp_input`, and `syslog_parser` into a single operator. ([PR24](https://github.com/open-telemetry/opentelemetry-log-collection/pull/24))
- Syslog operator RFC 3164 location parameter ([PR11](https://github.com/open-telemetry/opentelemetry-log-collection/pull/11))
- `uri_parser` operator ([PR12](https://github.com/open-telemetry/opentelemetry-log-collection/pull/12))

### Removed
- `forward_input` and `forward_output`, which were previously intended for use in the standalone agent ([PR27](https://github.com/open-telemetry/opentelemetry-log-collection/pull/27))

## [0.14.0] - 2021-02-02

### Changed
- Remove standalone agent functionality
- Simplifies modules
  - Combines `parser/syslog` and `input/windows` modules into the primary module
  - Removes output operators that were previously separate modules
  - Leaves `input/k8sevent` and `transformer/k8smetadata` as separate modules for now. These two have extensive dependencies and their usefulness in the collector needs to be discussed before merging or removing.

## [0.13.12] - 2021-01-26

### Changed
- Allow plugin parameters to have a default value even if they are required

## [0.13.11] - 2021-01-15

### Changed
- Updated version of stanza used in several isolated modules

## [0.13.10] - 2021-01-15

### Added
- `timestamp` parser now supports a `location` parameter

## [0.13.9] - 2021-01-04

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
