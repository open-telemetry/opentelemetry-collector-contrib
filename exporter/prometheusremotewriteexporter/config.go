// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"errors"
	"fmt"
	"net/textproto"
	"slices"

	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Remote Write exporter.
type Config struct {
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	BackOffConfig   configretry.BackOffConfig    `mapstructure:"retry_on_failure"`

	// prefix attached to each exported metric name
	// See: https://prometheus.io/docs/practices/naming/#metric-names
	Namespace string `mapstructure:"namespace"`

	// QueueConfig allows users to fine tune the queues
	// that handle outgoing requests.
	RemoteWriteQueue RemoteWriteQueue `mapstructure:"remote_write_queue"`

	// ExternalLabels defines a map of label keys and values that are allowed to start with reserved prefix "__"
	ExternalLabels map[string]string `mapstructure:"external_labels"`

	// Deprecated [v0.158.0]: configure client http settings under `http` block
	ClientConfig confighttp.ClientConfig `mapstructure:",squash"`

	// HTTP defines the HTTP client configuration in a nested block.
	// This field takes precedence over the squashed ClientConfig when set.
	HTTP confighttp.ClientConfig `mapstructure:"http"`

	// maximum size in bytes of time series batch sent to remote storage
	MaxBatchSizeBytes int `mapstructure:"max_batch_size_bytes"`

	// maximum amount of parallel requests to do when handling large batch request
	MaxBatchRequestParallelism *int `mapstructure:"max_batch_request_parallelism"`

	// ResourceToTelemetrySettings is the option for converting resource attributes to telemetry attributes.
	// "Enabled" - A boolean field to enable/disable this option. Default is `false`.
	// If enabled, all the resource attributes will be converted to metric labels by default.
	// "ExcludeServiceAttributes" - If set to `true`, the `service.name`, `service.instance.id` and `service.namespace` resource attributes,
	// which are already converted to `job` and `instance` labels respectively, will be excluded from the final metrics.
	//
	// Deprecated: Use ResourceConstantLabels instead.
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ResourceConstantLabels controls which resource attributes are added as constant labels on Prometheus metrics.
	ResourceConstantLabels resourcetotelemetry.Settings `mapstructure:"resource_constant_labels"`

	// WAL enables persisting metrics to a write-ahead-log before sending to the remote storage.
	WAL configoptional.Optional[WALConfig] `mapstructure:"wal"`

	// TargetInfo allows customizing the target_info metric
	TargetInfo TargetInfo `mapstructure:"target_info,omitempty"`

	// DisableScopeInfo allows disabling the export of the scope info labels
	DisableScopeInfo bool `mapstructure:"disable_scope_info"`

	// AddMetricSuffixes controls whether unit and type suffixes are added to metrics on export
	//
	// Deprecated: Use TranslationStrategy instead. It will be removed in v0.153.0.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`

	// TranslationStrategy controls how OTLP metric and attribute names are translated into Prometheus metric and label names.
	// When set, this takes precedence over AddMetricSuffixes.
	TranslationStrategy translationStrategy `mapstructure:"translation_strategy"`

	// SendMetadata controls whether prometheus metadata will be generated and sent, this option is ignored when using PRW 2.0, which always includes metadata.
	SendMetadata bool `mapstructure:"send_metadata"`

	// RemoteWriteProtoMsg controls whether prometheus remote write v1 or v2 is sent.
	RemoteWriteProtoMsg remoteapi.WriteMessageType `mapstructure:"protobuf_message,omitempty"`

	// IncludeMetadataKeys is a list of client metadata keys whose values are
	// forwarded as HTTP request headers on every remote write call.
	IncludeMetadataKeys []string `mapstructure:"include_metadata_keys"`

	// ConvertExplicitHistogramsToNHCB converts explicit-bucket histograms to NHCB (schema -53) instead of classic series.
	ConvertExplicitHistogramsToNHCB bool `mapstructure:"convert_explicit_histograms_to_nhcb"`

	// KeepClassicHistograms also emits the classic series alongside NHCB; no effect unless convert_explicit_histograms_to_nhcb is set.
	KeepClassicHistograms bool `mapstructure:"keep_classic_histograms"`
}

type translationStrategy string

const (
	// underscoreEscapingWithSuffixes escapes special characters to '_', and appends type and unit suffixes.
	underscoreEscapingWithSuffixes translationStrategy = "UnderscoreEscapingWithSuffixes"

	// underscoreEscapingWithoutSuffixes escapes special characters to '_', but suffixes won't be attached.
	underscoreEscapingWithoutSuffixes translationStrategy = "UnderscoreEscapingWithoutSuffixes"

	// noUTF8EscapingWithSuffixes does not change special characters to '_', but does append '_total' for counters and unit suffixes.
	noUTF8EscapingWithSuffixes translationStrategy = "NoUTF8EscapingWithSuffixes"

	// noTranslation passes metric and label names through unaltered.
	noTranslation translationStrategy = "NoTranslation"
)

type TargetInfo struct {
	// Enabled if false the target_info metric is not generated by the exporter
	Enabled bool `mapstructure:"enabled"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// RemoteWriteQueue allows to configure the remote write queue.
type RemoteWriteQueue struct {
	// Enabled if false the queue is not enabled, the export requests
	// are executed synchronously.
	Enabled bool `mapstructure:"enabled"`

	// QueueSize is the maximum number of OTLP metric batches allowed
	// in the queue at a given time. Ignored if Enabled is false.
	QueueSize int `mapstructure:"queue_size"`

	// NumWorkers configures the number of workers used by
	// the collector to fan out remote write requests.
	NumConsumers int `mapstructure:"num_consumers"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// TODO(jbd): Add capacity, max_samples_per_send to QueueConfig.

// reservedRemoteWriteHeaders are managed by the remote write protocol itself and
// must not be overwritten by headers forwarded from client metadata. Keys are
// stored in canonical MIME form for case-insensitive matching.
var reservedRemoteWriteHeaders = map[string]struct{}{
	textproto.CanonicalMIMEHeaderKey("Content-Encoding"):                  {},
	textproto.CanonicalMIMEHeaderKey("Content-Type"):                      {},
	textproto.CanonicalMIMEHeaderKey("User-Agent"):                        {},
	textproto.CanonicalMIMEHeaderKey("X-Prometheus-Remote-Write-Version"): {},
}

var _ component.Config = (*Config)(nil)

var topLevelHTTPClientKeys = []string{
	"endpoint",
	"proxy_url",
	"tls",
	"headers",
	"auth",
	"compression",
	"compression_params",
	"read_buffer_size",
	"write_buffer_size",
	"max_idle_conns",
	"max_idle_conns_per_host",
	"max_conns_per_host",
	"idle_conn_timeout",
	"disable_keep_alives",
	"http2_read_idle_timeout",
	"http2_ping_timeout",
	"cookies",
	"middlewares",
	"force_attempt_http2",
}

func hasTopLevelHTTPClientSettings(conf *confmap.Conf) bool {
	return slices.ContainsFunc(topLevelHTTPClientKeys, func(key string) bool {
		return conf.IsSet(key)
	})
}

// Unmarshal unmarshals the configuration and handles HTTP config precedence.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}

	if !conf.IsSet("http") && !metadata.ExporterPrometheusremotewritexporterRemoveTopLevelHTTPSettingsFeatureGate.IsEnabled() {
		cfg.HTTP = cfg.ClientConfig
		// we explicitly set an empty struct for TestLoadConfig to work. ClientConfig is not referenced outside tests
		cfg.ClientConfig = confighttp.ClientConfig{}
	}

	if metadata.ExporterPrometheusremotewritexporterRemoveTopLevelHTTPSettingsFeatureGate.IsEnabled() {
		// When the remove-top-level gate is enabled, reject deprecated flat HTTP client keys.
		if hasTopLevelHTTPClientSettings(conf) {
			return fmt.Errorf("top-level HTTP client settings are not allowed when feature gate %s is enabled; move them under the 'http' block",
				metadata.ExporterPrometheusremotewritexporterRemoveTopLevelHTTPSettingsFeatureGate.ID())
		}
	}

	return nil
}

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	//nolint:staticcheck // check deprecated fields
	if cfg.ResourceConstantLabels.Enabled || cfg.ResourceConstantLabels.ExcludeServiceAttributes {
		return errors.New("enabled and exclude_service_attributes are not supported under resource_constant_labels; use included and excluded instead")
	}
	if metadata.ExporterPrometheusremotewriteDisableResourceToTelemetryConversionFeatureGate.IsEnabled() {
		if !cfg.ResourceToTelemetrySettings.IsEmpty() {
			return errors.New("resource_to_telemetry_conversion is disabled by the exporter.prometheusremotewrite.DisableResourceToTelemetryConversion feature gate; use resource_constant_labels instead")
		}
	} else if !cfg.ResourceToTelemetrySettings.IsEmpty() && !cfg.ResourceConstantLabels.IsEmpty() {
		return errors.New("cannot configure both resource_to_telemetry_conversion and resource_constant_labels; resource_to_telemetry_conversion is deprecated")
	}
	if err := cfg.ResourceToTelemetrySettings.Validate(); err != nil {
		return err
	}
	if err := cfg.ResourceConstantLabels.Validate(); err != nil {
		return err
	}

	if cfg.MaxBatchRequestParallelism != nil && *cfg.MaxBatchRequestParallelism < 1 {
		return errors.New("max_batch_request_parallelism can't be set to below 1")
	}

	if cfg.RemoteWriteQueue.QueueSize < 0 {
		return errors.New("remote write queue size can't be negative")
	}

	if cfg.RemoteWriteQueue.Enabled && cfg.RemoteWriteQueue.QueueSize == 0 {
		return errors.New("a 0 size queue will drop all the data")
	}

	if cfg.RemoteWriteQueue.NumConsumers < 0 {
		return errors.New("remote write consumer number can't be negative")
	}

	if cfg.MaxBatchSizeBytes < 0 {
		return errors.New("max_batch_size_bytes must be greater than 0")
	}
	if cfg.MaxBatchSizeBytes == 0 {
		// Defaults to ~2.81MB
		cfg.MaxBatchSizeBytes = 3000000
	}

	if len(cfg.HTTP.Compression) > 0 && cfg.HTTP.Compression != "snappy" {
		return errors.New("compression type must be snappy")
	}

	err := cfg.RemoteWriteProtoMsg.Validate()
	if err != nil {
		return err
	}

	if !metadata.ExporterPrometheusremotewritexporterEnableSendingRW2FeatureGate.IsEnabled() && cfg.RemoteWriteProtoMsg == remoteapi.WriteV2MessageType {
		return fmt.Errorf("remote write v2 is only supported with the feature gate %s", metadata.ExporterPrometheusremotewritexporterEnableSendingRW2FeatureGate.ID())
	}

	// Validate translation strategy if set
	if cfg.TranslationStrategy != "" {
		switch cfg.TranslationStrategy {
		case underscoreEscapingWithSuffixes, underscoreEscapingWithoutSuffixes, noUTF8EscapingWithSuffixes, noTranslation:
		default:
			return fmt.Errorf("invalid translation_strategy: %s", cfg.TranslationStrategy)
		}

		if cfg.RemoteWriteProtoMsg == remoteapi.WriteV1MessageType && (cfg.TranslationStrategy == noUTF8EscapingWithSuffixes || cfg.TranslationStrategy == noTranslation) {
			return fmt.Errorf("translation strategy %s requires Prometheus Remote Write 2.0 (UTF-8 support)", cfg.TranslationStrategy)
		}
	}

	for _, key := range cfg.IncludeMetadataKeys {
		if _, reserved := reservedRemoteWriteHeaders[textproto.CanonicalMIMEHeaderKey(key)]; reserved {
			return fmt.Errorf("include_metadata_keys entry %q collides with a reserved remote write header", key)
		}
	}

	return nil
}
