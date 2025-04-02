// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// This mimics the original translation_rules config option (now deleted), but is not reachable
// by user configuration. It's only used by defaultTranslationRules to parse the
// YAML into a []translation.Rule object.
type translationRulesConfig struct {
	TranslationRules []translation.Rule `mapstructure:"translation_rules"`
}

var defaultTranslationRules = func() []translation.Rule {
	var data map[string]any
	var defaultRules translationRulesConfig

	// It is safe to panic since this is deterministic, and will not fail anywhere else if it doesn't fail all the time.
	if err := yaml.Unmarshal([]byte(translation.DefaultTranslationRulesYaml), &data); err != nil {
		panic(err)
	}

	if err := confmap.NewFromStringMap(data).Unmarshal(&defaultRules); err != nil {
		panic(fmt.Errorf("failed to load default translation rules: %w", err))
	}

	return defaultRules.TranslationRules
}()

var defaultExcludeMetrics = func() []dpfilters.MetricFilter {
	cfg, err := loadConfig([]byte(translation.DefaultExcludeMetricsYaml))
	// It is safe to panic since this is deterministic, and will not fail anywhere else if it doesn't fail all the time.
	if err != nil {
		panic(err)
	}
	return cfg.ExcludeMetrics
}()

var _ confmap.Unmarshaler = (*Config)(nil)

// Config defines configuration for SignalFx exporter.
type Config struct {
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	confighttp.ClientConfig   `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken configopaque.String `mapstructure:"access_token"`

	// Realm is the SignalFx realm where data is going to be sent to.
	Realm string `mapstructure:"realm"`

	// IngestURL is the destination to where SignalFx metrics will be sent to, it is
	// intended for tests and debugging. The value of Realm is ignored if the
	// URL is specified. The exporter will automatically append the appropriate
	// path: "/v2/datapoint" for metrics, and "/v2/event" for events.
	IngestURL string `mapstructure:"ingest_url"`

	// ingest_tls needs to be set if the exporter's IngestURL is pointing to a signalfx receiver
	// with TLS enabled and using a self-signed certificate where its CA is not loaded in the system cert pool.
	IngestTLSSettings configtls.ClientConfig `mapstructure:"ingest_tls,omitempty"`

	// APIURL is the destination to where SignalFx metadata will be sent. This
	// value takes precedence over the value of Realm
	APIURL string `mapstructure:"api_url"`

	// api_tls needs to be set if the exporter's APIURL is pointing to a httpforwarder extension
	// with TLS enabled and using a self-signed certificate where its CA is not loaded in the system cert pool.
	APITLSSettings configtls.ClientConfig `mapstructure:"api_tls,omitempty"`

	// Whether to log datapoints dispatched to Splunk Observability Cloud
	LogDataPoints bool `mapstructure:"log_data_points"`

	// Whether to log dimension updates being sent to SignalFx.
	LogDimensionUpdates bool `mapstructure:"log_dimension_updates"`

	// Dimension update client configuration used for metadata updates.
	DimensionClient DimensionClientConfig `mapstructure:"dimension_client"`

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`

	DisableDefaultTranslationRules bool `mapstructure:"disable_default_translation_rules"`

	// DeltaTranslationTTL specifies in seconds the max duration to keep the most recent datapoint for any
	// `delta_metric` specified in TranslationRules. Default is 3600s.
	DeltaTranslationTTL int64 `mapstructure:"delta_translation_ttl"`

	// SyncHostMetadata defines if the exporter should scrape host metadata and
	// sends it as property updates to SignalFx backend.
	// IMPORTANT: Host metadata synchronization relies on `resourcedetection` processor.
	//            If this option is enabled make sure that `resourcedetection` processor
	//            is enabled in the pipeline with one of the cloud provider detectors
	//            or environment variable detector setting a unique value to
	//            `host.name` attribute within your k8s cluster. Also keep override
	//            And keep `override=true` in resourcedetection config.
	SyncHostMetadata bool `mapstructure:"sync_host_metadata"`

	// ExcludeMetrics defines dpfilter.MetricFilters that will determine metrics to be
	// excluded from sending to SignalFx backend. If translations enabled with
	// TranslationRules options, the exclusion will be applied on translated metrics.
	ExcludeMetrics []dpfilters.MetricFilter `mapstructure:"exclude_metrics"`

	// IncludeMetrics defines dpfilter.MetricFilters to override exclusion any of metric.
	// This option can be used to included metrics that are otherwise dropped by default.
	// See ./translation/default_metrics.go for a list of metrics that are dropped by default.
	IncludeMetrics []dpfilters.MetricFilter `mapstructure:"include_metrics"`

	// ExcludeProperties defines dpfilter.PropertyFilters to prevent inclusion of
	// properties to include with dimension updates to the SignalFx backend.
	ExcludeProperties []dpfilters.PropertyFilter `mapstructure:"exclude_properties"`

	// Correlation configuration for syncing traces service and environment to metrics.
	Correlation *correlation.Config `mapstructure:"correlation"`

	// NonAlphanumericDimensionChars is a list of allowable characters, in addition to alphanumeric ones,
	// to be used in a dimension key.
	NonAlphanumericDimensionChars string `mapstructure:"nonalphanumeric_dimension_chars"`

	// Whether to drop histogram bucket metrics dispatched to Splunk Observability.
	// Default value is set to false.
	DropHistogramBuckets bool `mapstructure:"drop_histogram_buckets"`

	// Whether to send histogram metrics in OTLP format to Splunk Observability.
	// Default value is set to false.
	SendOTLPHistograms bool `mapstructure:"send_otlp_histograms"`
}

type DimensionClientConfig struct {
	MaxBuffered         int           `mapstructure:"max_buffered"`
	SendDelay           time.Duration `mapstructure:"send_delay"`
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `mapstructure:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `mapstructure:"idle_conn_timeout"`
	Timeout             time.Duration `mapstructure:"timeout"`
}

func (cfg *Config) getMetricTranslator(done chan struct{}) (*translation.MetricTranslator, error) {
	var rules []translation.Rule
	// The new way to disable default translation rules. This override any setting of the default TranslationRules.
	if cfg.DisableDefaultTranslationRules {
		rules = []translation.Rule{}
	} else {
		rules = defaultTranslationRules
	}
	metricTranslator, err := translation.NewMetricTranslator(rules, cfg.DeltaTranslationTTL, done)
	if err != nil {
		return nil, fmt.Errorf("invalid default translation rules: %w", err)
	}

	return metricTranslator, nil
}

func (cfg *Config) getIngestURL() (*url.URL, error) {
	strURL := cfg.IngestURL
	if cfg.IngestURL == "" {
		strURL = fmt.Sprintf("https://ingest.%s.signalfx.com", cfg.Realm)
	}

	ingestURL, err := url.Parse(strURL)
	if err != nil {
		return nil, fmt.Errorf("invalid \"ingest_url\": %w", err)
	}
	return ingestURL, nil
}

func (cfg *Config) getAPIURL() (*url.URL, error) {
	strURL := cfg.APIURL
	if cfg.APIURL == "" {
		strURL = fmt.Sprintf("https://api.%s.signalfx.com", cfg.Realm)
	}

	apiURL, err := url.Parse(strURL)
	if err != nil {
		return nil, fmt.Errorf("invalid \"api_url\": %w", err)
	}
	return apiURL, nil
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	return setDefaultExcludes(cfg)
}

// Validate checks if the exporter configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.AccessToken == "" {
		return errors.New(`requires a non-empty "access_token"`)
	}

	if cfg.Realm == "" && (cfg.IngestURL == "" || cfg.APIURL == "") {
		return errors.New(`requires a non-empty "realm", or` +
			` "ingest_url" and "api_url" should be explicitly set`)
	}

	if cfg.ClientConfig.Timeout < 0 {
		return errors.New(`cannot have a negative "timeout"`)
	}

	return nil
}

func setDefaultExcludes(cfg *Config) error {
	// If ExcludeMetrics is not set to empty, append defaults.
	if cfg.ExcludeMetrics == nil || len(cfg.ExcludeMetrics) > 0 {
		cfg.ExcludeMetrics = append(cfg.ExcludeMetrics, defaultExcludeMetrics...)
	}
	return nil
}

func loadConfig(bytes []byte) (Config, error) {
	var cfg Config
	var data map[string]any
	if err := yaml.Unmarshal(bytes, &data); err != nil {
		return cfg, err
	}

	if err := confmap.NewFromStringMap(data).Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to load default exclude metrics: %w", err)
	}

	return cfg, nil
}
