# Changelog

## Unreleased

## v0.37.1

## 🧰 Bug fixes 🧰

- Fixes a problem with v0.37.0 which contained dependencies on v0.36.0 components. They should have been updated to v0.37.0.

## v0.37.0

## 🚀 New components 🚀

- [`journald` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/journaldreceiver) to parse Journald events from systemd journal using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library

## 🛑 Breaking changes 🛑

- Remove squash on configtls.TLSClientSetting for splunkhecexporter (#5541)
- Remove squash on configtls.TLSClientSetting for elastic components (#5539)
- Remove squash on configtls.TLSClientSetting for observiqexporter (#5540)
- Remove squash on configtls.TLSClientSetting for AWS components (#5454)
- Move `k8sprocessor` to `k8sattributesprocessor`.
- Rename `k8s_tagger` configuration `k8sattributes`.
- filelog receiver: use empty value for `SeverityText` field instead of `"Undefined"` (#5423)
- Rename `configparser.ConfigMap` to `config.Map`
- Rename `pdata.AggregationTemporality*` to `pdata.MetricAggregationTemporality*`
- Remove deprecated `batchpertrace` package/module (#5380)

## 💡 Enhancements 💡

- `k8sattributes` processor: add container metadata enrichment (#5467, #5572)
- `resourcedetection` processor: Add an option to force using hostname instead of FQDN (#5064)
- `dockerstats` receiver: Move docker client into new shared `internal/docker` (#4702)
- `spanmetrics` processor:
  - Add exemplars to metrics (#5263)
  - Support resource attributes in metrics dimensions (#4624)
- `filter` processor:
  - Add log filtering by `regexp` type filters (#5237)
  - Add record level log filtering (#5418)
- `dynatrace` exporter: Handle non-gauge data types (#5056)
- `datadog` exporter:
  - Add support for exporting histograms as sketches (#5082)
  - Scrub sensitive information from errors (#5575)
  - Add option to send instrumentation library metadata tags with metrics (#5431)
- `podman` receiver: Add `api_version`, `ssh_key`, and `ssh_passphrase` config options (#5430)
- `signalfx` exporter:
  - Add `max_connections` config option (#5432)
  - Add dimension name to log when value > 256 chars (#5258)
  - Discourage setting of endpoint path (#4851)
- `kubeletstats` receiver: Convert to pdata instead of using OpenCensus (#5458)
- `tailsampling` processor: Add `invert_match` config option to `string_attribute` policy (#4393)
- `awsemf` exporter: Add a feature flag in UserAgent for AWS backend to monitor the adoptions (#5178)
- `splunkhec` exporter: Handle explicitly NaN and Inf values (#5581)
- `hostmetrics` receiver:
  - Collect more process states in processes scraper (#4856)
  - Add device label to paging scraper (#4854)
- `awskinesis` exporter: Extend to allow for dynamic export types (#5440)

## 🧰 Bug fixes 🧰

- `datadog` exporter:
  - Fix tags on summary and bucket metrics (#5416)
  - Fix cache key generation for cumulative metrics (#5417)
- `resourcedetection` processor: Fix failure to start collector if at least one detector returns an error (#5242)
- `prometheus` exporter: Do not record obsreport calls (#5438)
- `prometheus` receiver: Metric type fixes to match Prometheus functionality (#4865)
- `sentry` exporter: Fix sentry tracing (#4320)
- `statsd` receiver: Set quantiles for metrics (#5647)

## v0.36.0

## 🛑 Breaking changes 🛑

- `filter` processor: The configs for `logs` filter processor have been changed to be consistent with the `metrics` filter processor. (#4895)
- `splunk_hec` receiver: 
  - `source_key`, `sourcetype_key`, `host_key` and `index_key` have now moved under `hec_metadata_to_otel_attrs` (#4726)
  - `path` field on splunkhecreceiver configuration is removed: We removed the `path` attribute as any request going to the Splunk HEC receiver port should be accepted, and added the `raw_path` field to explicitly map the path accepting raw HEC data. (#4951)
- feat(dynatrace): tags is deprecated in favor of default_dimensions (#5055)

## 💡 Enhancements 💡

- `filter` processor: Add ability to `include` logs based on resource attributes in addition to excluding logs based on resource attributes for strict matching. (#4895)
- `kubelet` API: Add ability to create an empty CertPool when the system run environment is windows
- `JMX` receiver: Allow JMX receiver logging level to be configured (#4898)
- `datadog` exporter: Export histograms as in OpenMetrics Datadog check (#5065)
- `dockerstats` receiver: Set Schema URL (#5239)
- Rename memorylimiter -> memorylimiterprocessor (#5262)
- `awskinesis` exporter: Refactor AWS kinesis exporter to be synchronous  (#5248)

## v0.35.0

## 🛑 Breaking changes 🛑

- Rename configparser.Parser to configparser.ConfigMap (#5070)
- Rename TelemetryCreateSettings -> TelemetrySettings (#5169)

## 💡 Enhancements 💡

- chore: update influxdb exporter and receiver (#5058)
- chore(dynatrace): use payload limit from api constants (#5077)
- Add documentation for filelog's new force_flush_period parameter (#5066)
- Reuse the gzip reader with a sync.Pool (#5145)
- Add a trace observer when splunkhecreceiver is used for logs (#5063)
- Remove usage of deprecated pdata.AttributeValueMapToMap (#5174)
- Podman Stats Receiver: Receiver and Metrics implementation (#4577)

## 🧰 Bug fixes 🧰

- Use staleness markers generated by prometheus, rather than making our own (#5062)
- `datadogexporter` exporter: skip NaN and infinite values (#5053)

## v0.34.0

## 🚀 New components 🚀

- [`cumulativetodelta` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor) to convert cumulative sum metrics to cumulative delta

- [`file` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`jaeger` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/jaegerexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`kafka` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/kafkaexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`opencensus` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/opencensusexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`prometheus` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`prometheusremotewrite` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusremotewriteexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`zipkin` exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/zipkinexporter) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`attribute` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributeprocessor) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`filter` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`probabilisticsampler` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`resource` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`span` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/spanprocessor) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`hostmetrics` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`jaeger` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/jaegerreceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`kafka` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kafkareceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`opencensus` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/opencensusreceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`prometheus` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`zipkin` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/zipkinreceiver) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`bearertokenauth` extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/bearertokenauthextension) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`healthcheck` extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`oidcauth` extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/oidcauthextension) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`pprof` extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/pprofextension) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))
- [`testbed`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/testbed) from core repository ([#3474](https://github.com/open-telemetry/opentelemetry-collector/issues/3474))

## 💡 Enhancements 💡

- `tailsampling` processor: Add new policy `probabilistic` (#3876)

## v0.33.0

# 🎉 OpenTelemetry Collector Contrib v0.33.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.32.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- [`cumulativetodelta` processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor) to convert cumulative sum metrics to cumulative delta

## 💡 Enhancements 💡

- Collector contrib has now full support for metrics proto v0.9.0.

## v0.32.0

# 🎉 OpenTelemetry Collector Contrib v0.32.0 (Beta) 🎉

This release is marked as "bad" since the metrics pipelines will produce bad data.

- See https://github.com/open-telemetry/opentelemetry-collector/issues/3824

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.32.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- `splunk_hec` receiver/exporter: `com.splunk.source` field is mapped to `source` field in Splunk instead of `service.name` (#4596)
- `redis` receiver: Move interval runner package to `internal/interval` (#4600)
- `datadog` exporter: Export summary count and sum as monotonic counts (#4605)

## 💡 Enhancements 💡

- `logzio` exporter:
  - New implementation of an in-memory queue to store traces, data compression with gzip, and queue configuration options (#4395)
  - Make `Hclog2ZapLogger` struct and methods private for public go api review (#4431)
- `newrelic` exporter (#4392):
  - Marked unsupported metric as permanent error
  - Force the interval to be valid even if 0
- `awsxray` exporter: Add PHP stacktrace parsing support (#4454)
- `file_storage` extension: Implementation of batch storage API (#4145)
- `datadog` exporter:
  - Skip sum metrics with no aggregation temporality (#4597)
  - Export delta sums as counts (#4609)
- `elasticsearch` exporter: Add dedot support (#4579)
- `signalfx` exporter: Add process metric to translation rules (#4598)
- `splunk_hec` exporter: Add profiling logs support (#4464)
- `awsemf` exporter: Replace logGroup and logStream pattern with metric labels (#4466)

## 🧰 Bug fixes 🧰

- `awsxray` exporter: Fix the origin on ECS/EKS/EB on EC2 cases (#4391)
- `splunk_hec` exporter: Prevent re-sending logs that were successfully sent (#4467)
- `signalfx` exporter: Prefix temporary metric translations (#4394)

## v0.31.0

# 🎉 OpenTelemetry Collector Contrib v0.31.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.31.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- `influxdb` receiver: Removed `metrics_schema` config option (#4277)

## 💡 Enhancements 💡

- Update to OTLP 0.8.0:
  - Remove use of `IntHistogram` (#4276)
  - Update exporters/receivers for `NumberDataPoint`
- Remove use of deprecated `pdata` slice `Resize()` (#4203, #4208, #4209)
- `awsemf` exporter: Added the option to have a user who is sending metrics from EKS Fargate Container Insights to reformat them to look the same as insights from ECS so that they can be ingested by CloudWatch (#4130)
- `k8scluster` receiver: Support OpenShift cluster quota metrics (#4342)
- `newrelic` exporter (#4278):
  - Requests are now retry-able via configuration option (defaults to retries enabled). Permanent errors are not retried.
  - The exporter monitoring metrics now include an untagged summary metric for ease of use.
  - Improved error logging to include URLs that fail to post messages to New Relic.
- `datadog` exporter: Upscale trace stats when global sampling rate is set (#4213)

## 🧰 Bug fixes 🧰

- `statsd` receiver: Add option to set Counter to be monotonic (#4154)
- Fix `internal/stanza` severity mappings (#4315)
- `awsxray` exporter: Fix the wrong AWS env resource setting (#4384)
- `newrelic` exporter (#4278):
  - Configuration unmarshalling did not allow timeout value to be set to 0 in the endpoint specific section.
  - Request cancellation was not propagated via context into the http request.
  - The queued retry logger is set to a zap.Nop logger as intended.

## v0.30.0

# 🎉 OpenTelemetry Collector Contrib v0.30.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.30.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀
- `oauth2clientauth` extension: ported from core (#3848)
- `metrics-generation` processor: is now enabled and available (#4047) 

## 🛑 Breaking changes 🛑

- Removed `jaegerthrifthttp` exporter (#4089) 

## 💡 Enhancements 💡

- `tailsampling` processor:
  - Add new policy `status_code` (#3754)
  - Add new tail sampling processor policy: status_code (#3754)
- `awscontainerinsights` receiver:
  - Integrate components and fix bugs for EKS Container Insights (#3846) 
  - Add Cgroup to collect ECS instance metrics for container insights receiver #3875
- `spanmetrics` processor: Support sub-millisecond latency buckets (#4091) 
- `sentry` exporter: Add exception event capture in sentry (#3854)

## v0.29.0

# 🎉 OpenTelemetry Collector Contrib v0.29.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.29.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- `redis` receiver (#3808)
  - removed configuration `service_name`. Use resource processor or `resource_attributes` setting if using `receivercreator`
  - removed `type` label and set instrumentation library name to `otelcol/redis` as other receivers do

## 💡 Enhancements 💡

- `tailsampling` processor:
  - Add new policy `latency` (#3750)
  - Add new policy `status_code` (#3754)
- `splunkhec` exporter: Include `trace_id` and `span_id` if set (#3850)
- `newrelic` exporter: Update instrumentation naming in accordance with otel spec (#3733)
- `sentry` exporter: Added support for insecure connection with Sentry (#3446)
- `k8s` processor:
  - Add namespace k8s tagger (#3384)
  - Add ignored pod names as config parameter (#3520)
- `awsemf` exporter: Add support for `TaskDefinitionFamily` placeholder on log stream name (#3755)
- `loki` exporter: Add resource attributes as Loki label (#3418)

## 🧰 Bug fixes 🧰

- `datadog` exporter:
  - Ensure top level spans are computed (#3786)
  - Update `env` clobbering behavior (#3851)
- `awsxray` exporter: Fixed filtered attribute translation (#3757)
- `splunkhec` exporter: Include trace and span id if set in log record (#3850)

## v0.28.0

# 🎉 OpenTelemetry Collector Contrib v0.28.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.28.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `humio` exporter to export data to Humio using JSON over the HTTP [Ingest API](https://docs.humio.com/reference/api/ingest/)
- `udplog` receiver to receives logs from udp using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library
- `tanzuobservability` exporter to send traces to [Tanzu Observability](https://tanzu.vmware.com/observability)

## 🛑 Breaking changes 🛑

- `f5cloud` exporter (#3509):
  - Renamed the config 'auth' field to 'f5cloud_auth'. This will prevent a config field name collision when [Support for Custom Exporter Authenticators as Extensions](https://github.com/open-telemetry/opentelemetry-collector/pull/3128) is ready to be integrated.

## 💡 Enhancements 💡

- Enabled Dependabot for Github Actions (#3543)
- Change obsreport helpers for receivers to use the new pattern created in Collector (#3439,#3443,#3449,#3504,#3521,#3548)
- `datadog` exporter:
  - Add logging for unknown or unsupported metric types (#3421)
  - Add collector version tag to internal health metrics (#3394)
  - Remove sublayer stats calc and mutex (#3531)
  - Deduplicate hosts for which we send running metrics (#3539)
  - Add support for summary datatype (#3660)
  - Add datadog span operation name remapping config option (#3444)
  - Update error formatting for error spans that are not exceptions (#3701)
- `nginx` receiver: Update the nginx metrics to more closely align with the conventions (#3420)
- `elasticsearch` exporter: Init JSON encoding support (#3101)
- `jmx` receiver:
  - Allow setting system properties (#3450)
  - Update tested JMX Metric Gatherer release (#3695)
- Refactor components for the Client Authentication Extensions (#3507)
- Remove redundant conversion calls (#3688)
- `storage` extension: Add a `Close` method to Client interface (#3506)
- `splunkhec` exporter: Add `metric_type` as key which maps to the type of the metric (#3696)
- `k8s` processor: Add semantic conventions to k8s-tagger for pod metadata (#3544)
- `kubeletstats` receiver: Refactor kubelet client to internal folder (#3698)
- `newrelic` exporter (#3690):
  - Updates the log level from error to debug when New Relic rate limiting occurs
  - Updates the sanitized api key that is reported via metrics
- `filestorage` extension: Add ability to specify name (#3703)
- `awsemf` exporter: Store the initial value for cumulative metrics (#3425)
- `awskinesis` exporter: Refactor to allow for extended types of encoding (#3655)
- `ecsobserver` extension:
  - Add task definition, ec2, and service fetcher (#3503)
  - Add exporter to convert task to target (#3333)

## 🧰 Bug fixes 🧰

- `awsemf` exporter: Remove delta adjustment from summaries by default (#3408)
- `alibabacloudlogservice` exporter: Sanitize labels for metrics (#3454)
- `statsd` receiver: Fix StatsD drop metrics tags when using summary as observer_type for timer/histogram (#3440)
- `awsxray` exporter: Restore setting of Throttle for HTTP throttle response (#3685)
- `awsxray` receiver: Fix quick start bug (#3653)
- `metricstransform` processor: Check all data points for matching metric label values (#3435)

## v0.27.0

# 🎉 OpenTelemetry Collector Contrib v0.27.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.27.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `tcplog` receiver to receive logs from tcp using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library
- `influxdb` receiver to accept metrics data as [InfluxDB Line Protocol](https://docs.influxdata.com/influxdb/v2.0/reference/syntax/line-protocol/)

## 💡 Enhancements 💡

- `splunkhec` exporter:
  - Include the response in returned 400 errors (#3338)
  - Map summary metrics to Splunk HEC metrics (#3344)
  - Add HEC telemetry (#3260)
- `newrelic` exporter: Include dropped attributes and events counts (#3187)
- `datadog` exporter:
  - Add Fargate task ARN to container tags (#3326)
  - Improve mappings for span kind dd span type (#3368)
- `signalfx` exporter: Add info log for host metadata properties update (#3343)
- `awsprometheusremotewrite` exporter: Add SDK and system information to User-Agent header (#3317)
- `metricstransform` processor: Add filtering capabilities matching metric label values for applying changes (#3201)
- `groupbytrace` processor: Added workers for queue processing (#2902)
- `resourcedetection` processor: Add docker detector (#2775)
- `tailsampling` processor: Support regex on span attribute filtering (#3335)

## 🧰 Bug fixes 🧰

- `datadog` exporter:
  - Update Datadog attributes to tags mapping (#3292)
  - Consistent `hostname` and default metrics behavior (#3286)
- `signalfx` exporter: Handle character limits on metric names and dimensions (#3328)
- `newrelic` exporter: Fix timestamp value for cumulative metrics (#3406)

## v0.26.0

# 🎉 OpenTelemetry Collector Contrib v0.26.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.26.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `influxdb` exporter to support sending tracing, metrics, and logging data to [InfluxDB](https://www.influxdata.com/products/)

## 🛑 Breaking changes 🛑

- `signalfx` exporter (#3207):
  - Additional metrics excluded by default by signalfx exporter
    - system.disk.io_time
    - system.disk.operation_time
    - system.disk.weighted_io_time
    - system.network.connections
    - system.processes.count
    - system.processes.created

## 💡 Enhancements 💡

- Add default config and systemd environment file support for DEB/RPM packages (#3123)
- Log errors on receiver start/stop failures (#3208)
- `newrelic` exporter: Update API key detection logic (#3212)
- `splunkhec` exporter:
  - Mark permanent errors to avoid futile retries (#3253)
  - Add TLS certs verification (#3204)
- `datadog` exporter:
  - Add env and tag name normalization to trace payloads (#3200)
  - add `ignore_resource`s configuration option (#3245)
- `jmx` receiver: Update for latest snapshot and header support (#3283)
- `awsxray` exporter: Added support for stack trace translation for .NET language (#3280)
- `statsd` receiver: Add timing/histogram for statsD receiver as OTLP summary (#3261)

## 🧰 Bug fixes 🧰

- `awsprometheusremotewrite` exporter:
  - Remove `sending_queue` (#3186)
  - Use the correct default for aws_auth.service (#3161)
  - Identify the Amazon Prometheus region from the endpoint (#3210)
  - Don't panic in case session can't be constructed (#3221)
- `datadog` exporter: Add max tag length (#3185)
- `sapm` exporter: Fix crash when passing the signalfx access token (#3294)
- `newrelic` exporter: Update error conditions (#3322)

## v0.25.0

# 🎉 OpenTelemetry Collector Contrib v0.25.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.25.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `kafkametricsreceiver` new receiver component for collecting metrics about a kafka cluster - primarily lag and offset. [configuration instructions](receiver/kafkametricsreceiver/README.md)
- `file_storage` extension to read and write data to the local file system (#3087)

## 🛑 Breaking changes 🛑

- `newrelic` exporter (#3091):
  - Removal of common attributes (use opentelemetry collector resource processor to add attributes)
  - Drop support for cumulative metrics being sent to New Relic via a collector

## 💡 Enhancements 💡

- Update `opentelemetry-log-collection` to v0.17.0 for log receivers (#3017)
- `datadog` exporter:
  - Add `peer.service` priority instead of `service.name` (#2817)
  - Improve support of semantic conventions for K8s, Azure and ECS (#2623)
- Improve and batch logs translation for stanza (#2892)
- `statsd` receiver: Add timing/histogram as OTLP gauge (#2973)
- `honeycomb` exporter: Add Retry and Queue settings (#2714)
- `resourcedetection` processor:
  - Add AKS resource detector (#3035)
  - Use conventions package constants for ECS detector (#3171)
- `sumologic` exporter: Add graphite format (#2695)
- Add trace attributes to the log entry for stanza (#3018)
- `splunk_hec` exporter: Send log record name as part of the HEC log event (#3119)
- `newrelic` exporter (#3091):
  - Add support for logs
  - Performance improvements
  - Optimizations to the New Relic payload to reduce payload size
  - Metrics generated for monitoring the exporter
  - Insert Key vs License keys are auto-detected in some cases
  - Collector version information is properly extracted via the application start info parameters

## 🧰 Bug fixes 🧰

- `splunk_hec` exporter: Fix sending log payload with missing the GZIP footer (#3032)
- `awsxray` exporter: Remove propagation of error on shutdown (#2999)
- `resourcedetection` processor:
  - Correctly report DRAGONFLYBSD value (#3100)
  - Fallback to `os.Hostname` when FQDN is not available (#3099)
- `httpforwarder` extension: Do not report ErrServerClosed when shutting down the service (#3173)
- `collectd` receiver: Do not report ErrServerClosed when shutting down the service (#3178)

## v0.24.0

# 🎉 OpenTelemetry Collector Contrib v0.24.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.24.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `fluentbit` extension and `fluentforward` receiver moved from opentelemetry-collector

## 💡 Enhancements 💡

- Check `NO_WINDOWS_SERVICE` environment variable to force interactive mode on Windows (#2819)
- `resourcedetection `processor:
  - Add task revision to ECS resource detector (#2814)
  - Add GKE detector (#2821)
  - Add Amazon EKS detector (#2820)
  - Add `VMScaleSetName` field to Azure detector (#2890)
- `awsemf` exporter:
  - Add `parse_json_encoded_attr_values` config option to decode json-encoded strings in attribute values (#2827)
  - Add `output_destination` config option to support AWS Lambda (#2720)
- `googlecloud` exporter: Handle `cloud.availability_zone` semantic convention (#2893)
- `newrelic` exporter: Add `instrumentation.provider` to default attributes (#2900)
- Set unprivileged user to container image (#2925)
- `splunkhec` exporter: Add `max_content_length_logs` config option to send log data in payloads less than max content length (#2524)
- `k8scluster` and `kubeletstats` receiver: Replace package constants in favor of constants from conventions in core (#2996)

## 🧰 Bug fixes 🧰

- `spanmetrics` processor:
  - Rename `calls` metric to `calls_total` and set `IsMonotonic` to true (#2837)
  - Validate duplicate dimensions at start (#2844)
- `awsemf` exporter: Calculate delta instead of rate for cumulative metrics (#2512)
- `signalfx` exporter:
  - Remove more unnecessary translation rules (#2889)
  - Implement summary type (#2998)
- `awsxray` exporter: Remove translation to HTTP status from OC status (#2978)
- `awsprometheusremotewrite` exporter: Close HTTP body after RoundTrip (#2955)
- `splunkhec` exporter: Add ResourceAttributes to Splunk Event (#2843)

## v0.23.0

# 🎉 OpenTelemetry Collector Contrib v0.23.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.23.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `groupbyattrs` processor to group the records by provided attributes
- `dotnetdiagnostics` receiver to read metrics from .NET processes

## 🛑 Breaking changes 🛑

- `stackdriver` exporter marked as deprecated and renamed to `googlecloud`
- Change the rule expression in receiver creator for matching endpoints types from `type.port`, `type.hostport` and `type.pod` to `type == "port"`, `type == "hostport"` and `type == "pod"` (#2661)

## 💡 Enhancements 💡

- `loadbalancing` exporter: Add support for logs (#2470)
- `sumologic` exporter: Add carbon formatter (#2562)
- `awsecscontainermetrics` receiver: Add new metric for stopped container (#2383)
- `awsemf` exporter:
  - Send EMF logs in batches (#2572)
  - Add prometheus type field for CloudWatch compatibility (#2689)
- `signalfx` exporter:
  - Add resource attributes to events (#2631)
  - Add translation rule to drop dimensions (#2660)
  - Remove temporary host translation workaround (#2652)
  - Remove unnecessary default translation rules (#2672)
  - Update `exclude_metrics` option so that the default exclude rules can be overridden by setting the option to `[]` (#2737)
- `awsprometheusremotewrite` exporter: Add support for given IAM roles (#2675)
- `statsd` receiver: Change to use OpenTelemetry type instead of OpenCensus type (#2733)
- `resourcedetection` processor: Add missing entries for `cloud.infrastructure_service` (#2777)

## 🧰 Bug fixes 🧰

- `dynatrace` exporter: Serialize each datapoint into separate line (#2618)
- `splunkhec` exporter: Retain all otel attributes (#2712)
- `newrelic` exporter: Fix default metric URL (#2739)
- `googlecloud` exporter: Add host.name label if hostname is present in node (#2711)

## v0.22.0

# 🎉 OpenTelemetry Collector Contrib v0.22.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.22.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `filelog` receiver to tail and parse logs from files using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library

## 💡 Enhancements 💡

- `dynatrace` exporter: Send metrics to Dynatrace in chunks of 1000 (#2468)
- `k8s` processor: Add ability to associate metadata tags using pod UID rather than just IP (#2199)
- `signalfx` exporter:
  - Add statusCode to logging field on dimension client (#2459)
  - Add translation rules for `cpu.utilization_per_core` (#2540)
  - Updates to metadata handling (#2531)
  - Calculate extra network I/O metrics (#2553)
  - Calculate extra disk I/O metrics (#2557)
- `statsd` receiver: Add metric type label and `enable_metric_type` option (#2466)
- `sumologic` exporter: Add support for carbon2 format (#2562)
- `resourcedetection` processor: Add Azure detector (#2372)
- `k8scluster` receiver: Use OTel conventions for metadata (#2530)
- `newrelic` exporter: Multi-tenant support for sending trace data and performance enhancements (#2481)
- `stackdriver` exporter: Enable `retry_on_failure` and `sending_queue` options (#2613)
- Use standard way to convert from time.Time to proto Timestamp (#2548)

## 🧰 Bug fixes 🧰

- `signalfx` exporter:
  - Fix calculation of `network.total` metric (#2551)
  - Correctly convert dimensions on metadata updates (#2552)
- `awsxray` exporter and receiver: Fix the type of content_length (#2539)
- `resourcedetection` processor: Use values in accordance to semantic conventions for AWS (#2556)
- `awsemf` exporter: Fix concurrency issue (#2571)

## v0.21.0

# 🎉 OpenTelemetry Collector Contrib v0.21.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.21.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `loki` exporter to export data via HTTP to Loki

## 🛑 Breaking changes 🛑

- `signalfx` exporter: Allow periods to be sent in dimension keys (#2456). Existing users who do not want to change this functionality can set `nonalphanumeric_dimension_chars` to `_-`

## 💡 Enhancements 💡

- `awsemf` exporter:
  - Support unit customization before sending logs to AWS CloudWatch (#2318)
  - Group exported metrics by labels (#2317)
- `datadog` exporter: Add basic span events support (#2338)
- `alibabacloudlogservice` exporter: Support new metrics interface (#2280)
- `sumologic` exporter:
  - Enable metrics pipeline (#2117)
  - Add support for all types of log body (#2380)
- `signalfx` exporter: Add `nonalphanumeric_dimension_chars` config option (#2442)

## 🧰 Bug fixes 🧰

- `resourcedetection` processor: Fix resource attribute environment variable (#2378)
- `k8scluster` receiver: Fix nil pointer bug (#2450)

## v0.20.0

# 🎉 OpenTelemetry Collector Contrib v0.20.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.20.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `spanmetrics` processor to aggregate Request, Error and Duration (R.E.D) metrics from span data
- `awsxray` receiver to accept spans in the X-Ray Segment format
- `groupbyattrs` processor to group the records by provided attributes

## 🛑 Breaking changes 🛑

- Rename `kinesis` exporter to `awskinesis` (#2234)
- `signalfx` exporter: Remove `send_compatible_metrics` option, use `translation_rules` instead (#2267)
- `datadog` exporter: Remove default prefix from user metrics (#2308)

## 💡 Enhancements 💡

- `signalfx` exporter: Add k8s metrics to default excludes (#2167)
- `stackdriver` exporter: Reduce QPS (#2191)
- `datadog` exporter:
  - Translate otel exceptions to DataDog errors (#2195)
  - Use resource attributes for metadata and generated metrics (#2023)
- `sapm` exporter: Enable queuing by default (#1224)
- `dynatrace` exporter: Allow underscores anywhere in metric or dimension names (#2219)
- `awsecscontainermetrics` receiver: Handle stopped container's metadata (#2229)
- `awsemf` exporter: Enhance metrics batching in AWS EMF logs (#2271)
- `f5cloud` exporter: Add User-Agent header with version to requests (#2292)

## 🧰 Bug fixes 🧰

- `signalfx` exporter: Reinstate network/filesystem translation rules (#2171)

## v0.19.0

# 🎉 OpenTelemetry Collector Contrib v0.19.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.19.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `f5cloud` exporter to export metric, trace, and log data to F5 Cloud
- `jmx` receiver to report metrics from a target MBean server in conjunction with the [JMX Metric Gatherer](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/contrib/jmx-metrics/README.md)

## 🛑 Breaking changes 🛑

- `signalfx` exporter: The `exclude_metrics` option now takes slice of metric filters instead of just metric names (slice of strings) (#1951)

## 💡 Enhancements 💡

- `datadog` exporter: Sanitize datadog service names (#1982)
- `awsecscontainermetrics` receiver: Add more metadata (#2011)
- `azuremonitor` exporter: Favor RPC over HTTP spans (#2006)
- `awsemf` exporter: Always use float64 as calculated rate (#2019)
- `splunkhec` receiver: Make the HEC receiver path configurable, and use `/*` by default (#2137)
- `signalfx` exporter:
  - Drop non-default metrics and add `include_metrics` option to override (#2145, #2146, #2162)
  - Rename `system.network.dropped_packets` metric to `system.network.dropped` (#2160)
  - Do not filter cloud attributes from dimensions (#2020)
- `redis` receiver: Migrate to pdata metrics #1889

## 🧰 Bug fixes 🧰

- `datadog` exporter: Ensure that version tag is added to trace stats (#2010)
- `loadbalancing` exporter: Rolling update of collector can stop the periodical check of DNS updates (#1798)
- `awsecscontainermetrics` receiver: Change the type of `exit_code` from string to int and deal with the situation when there is no data (#2147)
- `groupbytrace` processor: Make onTraceReleased asynchronous to fix processor overload (#1808)
- Handle cases where the time field of Splunk HEC events is encoded as a String (#2159)

## v0.18.0

# 🎉 OpenTelemetry Collector Contrib v0.18.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.18.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `sumologic` exporter to send logs and metrics data to Sumo Logic
- `dynatrace` exporter to send metrics to Dynatrace

## 💡 Enhancements 💡

- `datadog` exporter:
  - Add resource attributes to tags conversion feature (#1782)
  - Add Kubernetes conventions for hostnames (#1919)
  - Add container tags to datadog export for container infra metrics in service view (#1895)
  - Update resource naming and span naming (#1861)
  - Add environment variables support for config options (#1897)
- `awsxray` exporter: Add parsing of JavaScript stack traces (#1888)
- `elastic` exporter: Translate exception span events (#1858)
- `signalfx` exporter: Add translation rules to aggregate per core CPU metrics in default translations (#1841)
- `resourcedetection` processor: Gather tags associated with the EC2 instance and add them as resource attributes (#1899)
- `simpleprometheus` receiver: Add support for passing params to the prometheus scrape config (#1949)
- `azuremonitor` exporter: Implement Span status code specification changes - gRPC (#1960)
- `metricstransform` processor: Add grouping option ($1887)
- `alibabacloudlogservice` exporter: Use producer to send data to improve performance (#1981)

## 🧰 Bug fixes 🧰

- `datadog` exporter: Handle monotonic metrics client-side (#1805)
- `awsxray` exporter: Log error when translating span (#1809)

## v0.17.0

# 🎉 OpenTelemetry Collector Contrib v0.17.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.17.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 💡 Enhancements 💡

- `awsemf` exporter: Add collector version to EMF exporter user agent (#1778)
- `signalfx` exporter: Add configuration for trace correlation (#1795)
- `statsd` receiver: Add support for metric aggregation (#1670)
- `datadog` exporter: Improve logging of hostname detection (#1796)

## 🧰 Bug fixes 🧰

- `resourcedetection` processor: Fix ecs detector to not use the default golang logger (#1745)
- `signalfx` receiver: Return 200 when receiver succeed (#1785)
- `datadog` exporter: Use a singleton for sublayer calculation (#1759)
- `awsxray` and `awsemf` exporters: Change the User-Agent content order (#1791)

## v0.16.0

# 🎉 OpenTelemetry Collector Contrib v0.16.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.16.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- `honeycomb` exporter: Update to use internal data format (#1689)

## 💡 Enhancements 💡

- `newrelic` exporter: Add support for span events (#1643)
- `awsemf` exporter:
  - Add placeholder support in `log_group_name` and `log_stream_name` config (#1623, #1661)
  - Add label matching filtering rule (#1619)
- `resourcedetection` processor: Add new resource detector for AWS Elastic Beanstalk environments (#1585)
- `loadbalancing` exporter:
  - Add sort of endpoints in static resolver (#1692)
  - Allow specifying port when using DNS resolver (#1650)
- Add `batchperresourceattr` helper library that splits an incoming data based on an attribute in the resource (#1694)
- `alibabacloudlogservice` exporter:
  - Add logs exporter (#1609)
  - Change trace type from opencensus to opentelemetry (#1713)
- `datadog` exporter:
  - Improve trace exporter performance (#1706, #1707)
  - Add option to only send metadata (#1723)
- `awsxray` exporter:
  - Add parsing of Python stack traces (#1676)
  - Add collector version to user agent (#1730)

## 🧰 Bug fixes 🧰

- `loadbalancing` exporter:
  - Fix retry queue for exporters (#1687)
  - Fix `periodicallyResolve` for DNS resolver checks (#1678)
- `datadog` exporter: Fix status code handling (#1691)
- `awsxray` exporter:
  - Fix empty traces in X-Ray console (#1709)
  - Stricter requirements for adding http request url (#1729)
  - Fix status code handling for errors/faults (#1740)
- `signalfx` exporter:
  - Split incoming data requests by access token before enqueuing (#1727)
  - Disable retry on 400 and 401, retry with backoff on 429 and 503 (#1672)
- `awsecscontainermetrics` receiver: Improve error handling to fix seg fault (#1738)

## v0.15.0

# 🎉 OpenTelemetry Collector Contrib v0.15.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.15.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `zookeeper` receiver: Collects metrics from a Zookeeper instance using the `mntr` command
- `loadbalacing` exporter: Consistently exports spans belonging to the same trace to the same backend
- `windowsperfcounters` receiver: Captures the configured system, application, or custom performance counter data from the Windows registry using the PDH interface
- `awsprometheusremotewrite` exporter:  Sends metrics data in Prometheus TimeSeries format to a Prometheus Remote Write Backend and signs each outgoing HTTP request following the AWS Signature Version 4 signing process

## 💡 Enhancements 💡

- `awsemf` exporter:
  - Add `metric_declarations` config option for metric filtering and dimensions (#1503)
  - Add SummaryDataType and remove Min/Max from Histogram (#1584)
- `signalfxcorrelation` exporter: Add ability to translate host dimension (#1561)
- `newrelic` exporter: Use pdata instead of the OpenCensus for traces (#1587)
- `metricstransform` processor:
  - Add `combine` action for matched metrics (#1506)
  - Add `submatch_case` config option to specify case of matched label values (#1640)
- `awsecscontainermetrics` receiver: Extract cluster name from ARN (#1626)
- `elastic` exporter: Improve handling of span status if the status code is unset (#1591)

## 🧰 Bug fixes 🧰

- `awsemf` exporter: Add check for unhandled metric data types (#1493)
- `groupbytrace` processor: Make buffered channel to avoid goroutines leak (#1505)
- `stackdriver` exporter: Set `options.UserAgent` so that the OpenCensus exporter does not override the UA ($1620)

## v0.14.0

# 🎉 OpenTelemetry Collector Contrib v0.14.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.14.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `datadog` exporter to send metric and trace data to Datadog (#1352)
- `tailsampling` processor moved from core to contrib (#1383)

## 🛑 Breaking changes 🛑

- `jmxmetricsextension` migrated to `jmxreceiver` (#1182, #1357)
- Move signalfx correlation code out of `sapm` to `signalfxcorrelation` exporter (#1376)
- Move Splunk specific utils outside of common (#1306)
- `stackdriver` exporter:
    - Config options `metric_prefix` & `skip_create_metric_descriptor` are now nested under `metric`, see [README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/stackdriverexporter/README.md).
    - Trace status codes no longer reflect gRPC codes as per spec changes: open-telemetry/opentelemetry-specification#1067
- `datadog` exporter: Remove option to change the namespace prefix (#1483)

## 💡 Enhancements 💡

- `splunkhec` receiver: Add ability to ingest metrics (#1276)
- `signalfx` receiver: Improve pipeline error handling (#1329)
- `datadog` exporter:
  - Improve hostname resolution (#1285)
  - Add flushing/export of traces and trace-related statistics (#1266)
  - Enable traces on Windows (#1340)
  - Send otel.exporter running metric (#1354)
  - Add tag normalization util method (#1373)
  - Send host metadata (#1351)
  - Support resource conventions for hostnames (#1434)
  - Add version tag extract (#1449)
- Add `batchpertrace` library to split the incoming batch into several batches, one per trace (#1257)
- `statsd` receiver:
  - Add timer support (#1335)
  - Add sample rate support for counter, transfer gauge to double and transfer counter to int only (#1361)
- `awsemf` exporter: Restructure metric translator logic (#1353)
- `resourcedetection` processor:
  - Add EC2 hostname attribute (#1324)
  - Add ECS Resource detector (#1360)
- `sapm` exporter: Add queue settings (#1390)
- `metrictransform` processor: Add metric filter option (#1447)
- `awsxray` exporter: Improve ECS attribute and origin translation (#1428)
- `resourcedetection` processor: Initial system detector (#1405)

## 🧰 Bug fixes 🧰

- Remove duplicate definition of cloud providers with core conventions (#1288)
- `kubeletstats` receiver: Handle nil references from the kubelet API (#1326)
- `awsxray` receiver:
  - Add kind type to root span to fix the empty parentID problem (#1338)
  - Fix the race condition issue (#1490)
- `awsxray` exporter:
  - Setting the tlsconfig InsecureSkipVerify using NoVerifySSL (#1350)
  - Drop invalid xray trace id (#1366)
- `elastic` exporter: Ensure span name is limited (#1371)
- `splunkhec` exporter: Don't send 'zero' timestamps to Splunk HEC (#1157)
- `stackdriver` exporter: Skip processing empty metrics slice (#1494)

## v0.13.0

# 🎉 OpenTelemetry Collector Contrib v0.13.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.13.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 💡 Enhancements 💡

- `sapm` exporter:
  - Enable queuing by default (#1224)
  - Add SignalFx APM correlation (#1205)
  - Make span source attribute and destination dimension names configurable (#1286)
- `signalfx` exporter:
  - Pass context to the http client requests (#1225)
  - Update `disk.summary_utilization` translation rule to accommodate new labels (#1258)
- `newrelic` exporter: Add `span.kind` attribute (#1263)
- `datadog` exporter:
  - Add Datadog trace translation helpers (#1208)
  - Add API key validation (#1216)
- `splunkhec` receiver: Add the ability to ingest logs (#1268)
- `awscontainermetrics` receiver: Report `CpuUtilized` metric in percentage (#1283)
- `awsemf` exporter: Only calculate metric rate for cumulative counter and avoid SingleDimensionRollup for metrics with only one dimension (#1280)

## 🧰 Bug fixes 🧰

- Make `signalfx` exporter a metadata exporter (#1252)
- `awsecscontainermetrics` receiver: Check for empty network rate stats and set zero (#1260)
- `awsemf` exporter: Remove InstrumentationLibrary dimension in CloudWatch EMF Logs if it is undefined (#1256)
- `awsxray` receiver: Fix trace/span id transfer (#1264)
- `datadog` exporter: Remove trace support for Windows for now (#1274)
- `sapm` exporter: Correlation enabled check inversed (#1278)

## v0.12.0

# 🎉 OpenTelemetry Collector Contrib v0.12.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.12.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- `awsemf` exporter to support exporting metrics to AWS CloudWatch (#498, #1169)
- `http_forwarder` extension that forwards HTTP requests to a specified target (#979, #1014, #1150)
- `datadog` exporter that sends metric and trace data to Datadog (#1142, #1178, #1181, #1212)
- `awsecscontainermetrics` receiver to collect metrics from Amazon ECS Task Metadata Endpoint (#1089, #1148, #1160)

## 💡 Enhancements 💡

- `signalfx` exporter:
  - Add host metadata synchronization (#1039, #1118)
  - Add `copy_dimensions` translator option (#1126)
  - Update `k8s_cluster` metric translations (#1121)
  - Add option to exclude metrics (#1156)
  - Add `avg` aggregation method (#1151)
  - Fallback to host if cloud resource id not found (#1170)
  - Add backwards compatible translation rules for the `dockerstatsreceiver` (#1201)
  - Enable queuing and retries (#1223)
- `splunkhec` exporter:
  - Add log support (#875)
  - Enable queuing and retries (#1222)
- `k8scluster` receiver: Standardize metric names (#1119)
- `awsxray` exporter:
  - Support AWS EKS attributes (#1090)
  - Store resource attributes in X-Ray segments (#1174)
- `honeycomb` exporter:
  - Add span kind to the event sent to Honeycomb (#474)
  - Add option to adjust the sample rate using an attribute on the span (#1162)
- `jmxmetrics` extension: Add subprocess manager to manage child java processes (#1028)
- `elastic` exporter: Initial metrics support (#1173)
- `k8s` processor: Rename default attr names for label/annotation extraction (#1214)
- Add common SignalFx host id extraction (#1100)
- Allow MSI upgrades (#1165)

## 🧰 Bug fixes 🧰

- `awsxray` exporter: Don't set origin to EC2 when not on AWS (#1115)

## v0.11.0

# 🎉 OpenTelemetry Collector Contrib v0.11.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.11.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀
- add `dockerstats` receiver as top level component (#1081)
- add `tracegen` utility (#956)

## 💡 Enhancements 💡
- `stackdriver` exporter: Allow overriding client options via config (#1010)
- `k8scluster` receiver: Ensure informer caches are synced before initial data sync (#842)
- `elastic` exporter: Translate `deployment.environment` resource attribute to Elastic APM's semantically equivalent `service.environment` (#1022)
- `k8s` processor: Add logs support (#1051)
- `awsxray` exporter: Log response error with zap (#1050)
- `signalfx` exporter
  - Add dimensions to renamed metrics (#1041)
  - Add translation rules for `disk_ops.total` and `disk_ops.pending` metrics (#1082)
  - Add event support (#1036)
- `kubeletstats` receiver: Cache detailed PVC labels to reduce API calls (#1052)
- `signalfx` receiver: Add event support (#1035)

## v0.10.0

# 🎉 OpenTelemetry Collector Contrib v0.10.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.10.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀
- add initial docker stats receiver, without sourcing in top level components (#495)
- add initial jmx metrics extension structure, without sourcing in top level components (#740)
- `routing` processor for routing spans based on HTTP headers (#907)
- `splunkhec` receiver to receive Splunk HEC metrics, traces and logs (#840)
- Add skeleton for `http_forwarder` extension that forwards HTTP requests to a specified target (#979)

## 💡 Enhancements 💡
- `stackdriver` exporter
  - Add timeout parameter (#835)
  - Add option to configurably set UserAgent string (#758)
- `signalfx` exporter
  - Reduce memory allocations for big batches processing (#871)
  - Add AWSUniqueId and gcp_id generation (#829)
  - Calculate cpu.utilization compatibility metric (#839, #974, #954)
- `metricstransform` processor: Replace `{{version}}` in label values (#876)
- `resourcedetection` processor: Logs Support (#970)
- `statsd` receiver: Add parsing for labels and gauges (#903)

## 🧰 Bug fixes 🧰
- `k8s` processor
  - Wrap metrics before sending further down the pipeline (#837)
  - Fix setting attributes on metrics passed from agent (#836)
- `awsxray` exporter: Fix "pointer to empty string" is not omitted bug (#830)
- `azuremonitor` exporter: Treat UNSPECIFIED span kind as INTERNAL (#844)
- `signalfx` exporter: Remove misleading warnings (#869)
- `newrelic` exporter: Fix panic if service name is empty (#969)
- `honeycomb` exporter: Don't emit default proc id + starttime (#972)

## v0.9.0

# 🎉 OpenTelemetry Collector Contrib v0.9.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.9.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑
- Remove deprecated `lightstep` exporter (#828)

## 🚀 New components 🚀
- `statsd` receiver for ingesting StatsD messages (#566)

## 💡 Enhancements 💡
- `signalfx` exporter
   - Add disk usage translations (#760)
   - Add disk utilization translations (#782)
   - Add translation rule to drop redundant metrics (#809)
- `kubeletstats` receiver
  - Sync available volume metadata from /pods endpoint (#690)
  - Add ability to collect detailed data from PVC (#743)
- `awsxray` exporter: Translate SDK name/version into xray model (#755)
- `elastic` exporter: Translate semantic conventions to Elastic destination fields (#671)
- `stackdriver` exporter: Add point count metric (#757)
- `awsxray` receiver
  - Ported the TCP proxy from the X-Ray daemon (#774)
  - Convert to OTEL trace format (#691)

## 🧰 Bug fixes 🧰
- `kubeletstats` receiver: Do not break down metrics batch (#754)
- `host` observer: Fix issue on darwin where ports listening on all interfaces are not correctly accounted for (#582)
- `newrelic` exporter: Fix panic on missing span status (#775)

## v0.8.0

# 🎉 OpenTelemetry Collector Contrib v0.8.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.8.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- Receivers
  - `prometheusexec` subprocess manager (##499)

## 💡 Enhancements 💡

- `signalfx` exporter
  - Add/Update metric translations (#579, #584, #639, #640, #652, #662)
  - Add support for calculate new metric translator (#644)
  - Add renaming rules for load metrics (#664)
  - Update `container.name` to `k8s.container.name` in default translation rule (#683)
  - Rename working-set and page-fault metrics (#679)
- `awsxray` exporter
  - Translate exception event into xray exception (#577)
  - Add ingestion of X-Ray segments via UDP (#502)
  - Parse Java stacktrace and populate in xray cause (#687)
- `kubeletstats` receiver
  - Add metric_groups option (#648)
  - Set datapoint timestamp in receiver (#661)
  - Change `container.name` label to `k8s.container.name` (#680)
  - Add working-set and page-fault metrics (#666)
  - Add basic support for volume metrics (#667)
- `stackdriver` trace exporter: Move to new interface and pdata (#486)
- `metricstranform` processor: Keep timeseries and points in order after aggregation (#663)
- `k8scluster` receiver: Change `container.spec.name` label to `k8s.container.name` (#681)
- Migrate receiver creator to internal data model (#701)
- Add ec2 support to `resourcedetection` processor (#587)
- Enable timeout, sending queue and retry for SAPM exporter (#707)

## 🧰 Bug fixes 🧰

- `azuremonitor` exporter: Correct HTTP status code success mapping (#588)
- `k8scluster` receiver: Fix owner reference in metadata updates (#649)
- `awsxray` exporter: Fix handling of db system (#697)

## 🚀 New components 🚀

- Skeleton for AWS ECS container metrics receiver (#463)
- `prometheus_exec` receiver (#655)

## v0.7.0

# 🎉 OpenTelemetry Collector Contrib v0.7.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.7.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- `awsxray` receiver updated to support udp: `tcp_endpoint` config option renamed to `endpoint` (#497)
- TLS config changed for `sapmreceiver` (#488) and `signalfxreceiver` receivers (#488)

## 🚀 New components 🚀

- Exporters
  - `sentry` adds tracing exporter for [Sentry](https://sentry.io/) (#565)
- Extensions
  - `endpoints` observer: adds generic endpoint watcher (#427)
  - `host` observer: looks for listening network endpoints on host (#432)

## 💡 Enhancements 💡

- Update `honeycomb` exporter for v0.8.0 compatibility
- Extend `metricstransform` processor to be able to add a label to an existing metric (#441)
- Update `kubeletstats` metrics according to semantic conventions (#475)
- Updated `awsxray` receiver config to use udp (#497)
- Add `/pods` endpoint support in `kubeletstats` receiver to add extra labels (#569)
- Add metric translation options to `signalfx` exporter (#477, #501, #571, #573)

## 🧰 Bug fixes 🧰

- `azuremonitor` exporter: Mark spanToEnvelope errors as permanent (#500)

## v0.6.0

# 🎉 OpenTelemetry Collector Contrib v0.6.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.6.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

- Removed `jaegarlegacy` (#397) and `zipkinscribe` receivers (#410)
- `kubeletstats` receiver: Renamed `k8s.pod.namespace` pod label to `k8s.namespace.name` and `k8s.container.name` container label to `container.name`

## 🚀 New components 🚀

- Processors
  - `metricstransform` renames/aggregates within individual metrics (#376) and allow changing the data type between int and float (#402)

## 💡 Enhancements 💡

- `awsxray` exporter: Use `peer.service` as segment name when set. (#385)
- `splunk` exporter: Add trace exports support (#359, #399)
- Build and publish Windows MSI (#408) and DEB/RPM Linux packages (#405)

## 🧰 Bug fixes 🧰

- `kubeletstats` receiver:
  - Fixed NPE for newly created pods (#404)
  - Updated to latest change in the ReceiverFactoryOld interface (#401)
  - Fixed logging and self reported metrics (#357)
- `awsxray` exporter: Only convert SQL information for SQL databases. (#379)
- `resourcedetection` processor: Correctly obtain machine-type info from gce metadata (#395)
- `k8scluster` receiver: Fix container resource metrics (#416)

## v0.5.0

Released 01-07-2020

# 🎉 OpenTelemetry Collector Contrib v0.5.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.5.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🚀 New components 🚀

- Processors
  - `resourcedetection` to automatically detect the resource based on the configured set of detectors (#309)

## 💡 Enhancements 💡

- `kubeletstats` receiver: Support for ServiceAccount authentication (#324)
- `signalfx` exporter and receiver
  - Add SignalFx metric token passthrough and config option (#325)
  - Set default endpoint of `signalfx` receiver to `:9943` (#351)
- `awsxray` exporter: Support aws plugins EC2/ECS/Beanstalk (#343)
- `sapm` exporter and receiver: Add SAPM access token passthrough and config option (#349)
- `k8s` processor: Add metrics support (#358)
- `k8s` observer: Separate annotations from labels in discovered pods (#363)

## 🧰 Bug fixes 🧰

- `honeycomb` exporter: Remove shared use of libhoney from goroutines (#305)

## v0.4.0

Released 17-06-2020

# 🎉 OpenTelemetry Collector Contrib v0.4.0 (Beta) 🎉

The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.4.0) (be sure to check the release notes here as well!). Check out the [Getting Started Guide](https://opentelemetry.io/docs/collector/getting-started/) for deployment and configuration information.

## 🛑 Breaking changes 🛑

  - `signalfx` exporter `url` parameter changed to `ingest_url` (no impact if only using `realm` setting)

## 🚀 New components 🚀

- Receivers
  - `receiver_creator` to create receivers at runtime (#145), add observer support to receiver_creator (#173), add rules support (#207), add dynamic configuration values (#235) 
  - `kubeletstats` receiver (#237) 
  - `prometheus_simple` receiver (#184) 
  - `kubernetes-cluster` receiver (#175) 
  - `redis` receiver (#138)
- Exporters
  - `alibabacloudlogservice` exporter (#259) 
  - `SplunkHEC` metrics exporter (#246)
  - `elastic` APM exporter (#240)
  - `newrelic` exporter (#229) 
- Extensions
  - `k8s` observer (#185) 

## 💡 Enhancements 💡

- `awsxray` exporter
  - Use X-Ray convention of segment name == service name (#282)
  - Tweak xray export to improve rendering of traces and improve parity (#241)
  - Add handling for spans received with nil attributes (#212)
- `honeycomb` exporter
  - Use SendPresampled (#291)
  - Add span attributes as honeycomb event fields (#271)
  - Support resource labels in Honeycomb exporter (#20)
- `k8s` processor
  - Add support of Pod UID extraction to k8sprocessor (#219)
  - Use `k8s.pod.ip` to record resource IP instead of just `ip` (#183)
  - Support same authentication mechanism as other kubernetes components do (#307)
- `sapm` exporter: Add TLS for SAPM and SignalFx receiver (#215)
- `signalfx` exporter
  - Add metric metadata syncer to SignalFx exporter (#231)
  - Add TLS for SAPM and SignalFx receiver (#215)
- `stackdriver` exporter: Add support for resource mapping in config (#163)

## 🧰 Bug fixes 🧰

- `awsxray` exporter: Wrap bad request errors for proper handling by retry queue (#205)
- `lightstep` exporter: Ensure Lightstep exporter doesnt crash on nil node (#250)
- `sapm` exporter: Do not break Jaeger traces before sending downstream (#193)
- `k8s` processor: Ensure Jaeger spans work in passthrough mode (262)

## 🧩 Components 🧩

### Receivers

| Traces | Metrics |
|:-------:|:-------:|
| Jaeger Legacy | Carbon |
| SAPM (SignalFx APM) | Collectd | 
| Zipkin Scribe | K8s Cluster |
| | Redis |
| |  SignalFx | 
| | Simple Prometheus |
| | Wavefront |

### Processors

- K8s

### Exporters

| Commercial | Community |
|:------------:|:-----------:|
| Alibaba Cloud Log Service | Carbon |
| AWS X-ray | Elastic |
| Azure Monitor | Jaeger Thrift |
| Honeycomb | Kinesis |
| Lightstep |
| New Relic |
| SAPM (SignalFx APM) | 
| SignalFx (Metrics) |
| Splunk HEC |
| Stackdriver (Google) |

### Extensions

- Observer
  - K8s

## v0.3.0 Beta

Released 2020-03-30

### Breaking changes

-  Make prometheus receiver config loading strict. #697 
Prometheus receiver will now fail fast if the config contains unused keys in it.

### Changes and fixes

- Enable best effort serve by default of Prometheus Exporter (https://github.com/orijtech/prometheus-go-metrics-exporter/pull/6)
- Fix null pointer exception in the logging exporter #743 
- Remove unnecessary condition to have at least one processor #744 
- Updated Honeycomb exported to `honeycombio/opentelemetry-exporter-go v0.3.1`

### Features

Receivers / Exporters:

* AWS X-Ray
* Carbon
* CollectD
* Honeycomb
* Jaeger
* Kinesis
* LightStep
* OpenCensus
* OpenTelemetry
* SAPM
* SignalFx
* Stackdriver
* Wavefront
* Zipkin
* Zipkin Scribe


Processors:

* Attributes
* Batch
* Memory Limiter
* Queued Retry
* Resource
* Sampling
* Span
* Kubernetes

Extensions:

* Health Check
* Performance Profiler
* zPages


## v0.2.8

Released 2020-03-25

Alpha v0.2.8 of OpenTelemetry Collector Contrib.

- Implemented OTLP receiver and exporter.
- Added ability to pass config to the service programmatically (useful for custom builds).
- Improved own metrics / observability.


## v0.2.7

Released 2020-03-17

### Self-Observability
- New command-line switch to control legacy and new metrics. Users are encouraged
to experiment and migrate to the new metrics.
- Improved error handling on shutdown.


### Processors
- Fixed passthrough mode k8sprocessor.
- Added `HASH` action to attribute processor.

### Receivers and Exporters
- Added Honeycomb exporter.
- Added LightStep exporter.
- Added regular expression for Carbon receiver, allowing the metric name to be broken into proper label keys and values.
- Updated Stackdriver exporter to use a new batch API.


## v0.2.6 Alpha

Released 2020-02-18

### Self-Observability
- Updated metrics prefix to `otelcol` and expose command line argument to modify the prefix value.
- Batch dropped span now emits zero when no spans are dropped.

### Processors
- Extended Span processor to have include/exclude span logic.
- Ability to choose strict or regexp matching for include/exclude filters.

### Receivers and Exporters
- Added Carbon receiver and exporter.
- Added Wavefront receiver.


## v0.0.5 Alpha

Released 2020-01-30

- Regexp-based filtering of span names.
- Ability to extract attributes from span names and rename span.
- File exporter for debugging.
- Span processor is now enabled by default.

## v0.0.1 Alpha

Released 2020-01-11

First release of OpenTelemetry Collector Contrib.


[v0.3.0]: https://github.com/open-telemetry/opentelemetry-collector-contrib/compare/v0.2.8...v0.3.0
[v0.2.8]: https://github.com/open-telemetry/opentelemetry-collector-contrib/compare/v0.2.7...v0.2.8
[v0.2.7]: https://github.com/open-telemetry/opentelemetry-collector-contrib/compare/v0.2.6...v0.2.7
[v0.2.6]: https://github.com/open-telemetry/opentelemetry-collector-contrib/compare/v0.0.5...v0.2.6
[v0.0.5]: https://github.com/open-telemetry/opentelemetry-collector-contrib/compare/v0.0.1...v0.0.5
[v0.0.1]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.0.1
