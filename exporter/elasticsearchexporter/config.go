// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	QueueSettings exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	// Endpoints holds the Elasticsearch URLs the exporter should send events to.
	//
	// This setting is required if CloudID is not set and if the
	// ELASTICSEARCH_URL environment variable is not set.
	Endpoints []string `mapstructure:"endpoints"`

	// CloudID holds the cloud ID to identify the Elastic Cloud cluster to send events to.
	// https://www.elastic.co/guide/en/cloud/current/ec-cloud-id.html
	//
	// This setting is required if no URL is configured.
	CloudID string `mapstructure:"cloudid"`

	// NumWorkers configures the number of workers publishing bulk requests.
	NumWorkers int `mapstructure:"num_workers"`

	// LogsIndex configures the static index used for document routing for logs.
	// It should be empty if dynamic document routing is preferred.
	LogsIndex        string              `mapstructure:"logs_index"`
	LogsDynamicIndex DynamicIndexSetting `mapstructure:"logs_dynamic_index"`

	// MetricsIndex configures the static index used for document routing for metrics.
	// It should be empty if dynamic document routing is preferred.
	MetricsIndex        string              `mapstructure:"metrics_index"`
	MetricsDynamicIndex DynamicIndexSetting `mapstructure:"metrics_dynamic_index"`

	// TracesIndex configures the static index used for document routing for metrics.
	// It should be empty if dynamic document routing is preferred.
	TracesIndex        string              `mapstructure:"traces_index"`
	TracesDynamicIndex DynamicIndexSetting `mapstructure:"traces_dynamic_index"`

	// LogsDynamicID configures whether log record attribute `elasticsearch.document_id` is set as the document ID in ES.
	LogsDynamicID DynamicIDSettings `mapstructure:"logs_dynamic_id"`

	// LogsDynamicPipeline configures whether log record attribute `elasticsearch.document_pipeline` is set as the document ingest pipeline for ES.
	LogsDynamicPipeline DynamicPipelineSettings `mapstructure:"logs_dynamic_pipeline"`

	// Pipeline configures the ingest node pipeline name that should be used to process the
	// events.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest.html
	Pipeline string `mapstructure:"pipeline"`

	confighttp.ClientConfig `mapstructure:",squash"`
	Authentication          AuthenticationSettings `mapstructure:",squash"`
	Discovery               DiscoverySettings      `mapstructure:"discover"`
	Retry                   RetrySettings          `mapstructure:"retry"`
	Flush                   FlushSettings          `mapstructure:"flush"`
	Mapping                 MappingsSettings       `mapstructure:"mapping"`
	LogstashFormat          LogstashFormatSettings `mapstructure:"logstash_format"`

	// TelemetrySettings contains settings useful for testing/debugging purposes.
	// This is experimental and may change at any time.
	TelemetrySettings `mapstructure:"telemetry"`

	// IncludeSourceOnError configures whether the bulk index responses include
	// a part of the source document on error.
	// Defaults to nil.
	//
	// This setting requires Elasticsearch 8.18+. Using it in prior versions
	// have no effect.
	//
	// NOTE: The default behavior if this configuration is not set, is to
	// discard the error reason entirely, i.e. only the error type is returned.
	//
	// WARNING: If set to true, the exporter may log error responses containing
	// request payload, causing potential sensitive data to be exposed in logs.
	// Users are expected to sanitize the responses themselves.
	IncludeSourceOnError *bool `mapstructure:"include_source_on_error"`

	// Batcher holds configuration for batching requests based on timeout
	// and size-based thresholds.
	//
	// Batcher is unused by default, in which case Flush will be used.
	// If Batcher.Enabled is non-nil (i.e. batcher::enabled is specified),
	// then the Flush will be ignored even if Batcher.Enabled is false.
	Batcher BatcherConfig `mapstructure:"batcher"`
}

// BatcherConfig holds configuration for exporterbatcher.
//
// This is a slightly modified version of exporterbatcher.Config,
// to enable tri-state Enabled: unset, false, true.
type BatcherConfig struct {
	exporterhelper.BatcherConfig `mapstructure:",squash"`

	// enabledSet tracks whether Enabled has been specified.
	// If enabledSet is false, the exporter will perform its
	// own buffering.
	enabledSet bool `mapstructure:"-"`
}

func (c *BatcherConfig) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(c); err != nil {
		return err
	}
	c.enabledSet = conf.IsSet("enabled")
	return nil
}

type TelemetrySettings struct {
	LogRequestBody  bool `mapstructure:"log_request_body"`
	LogResponseBody bool `mapstructure:"log_response_body"`

	LogFailedDocsInput          bool          `mapstructure:"log_failed_docs_input"`
	LogFailedDocsInputRateLimit time.Duration `mapstructure:"log_failed_docs_input_rate_limit"`
}

type LogstashFormatSettings struct {
	Enabled         bool   `mapstructure:"enabled"`
	PrefixSeparator string `mapstructure:"prefix_separator"`
	DateFormat      string `mapstructure:"date_format"`
}

type DynamicIndexSetting struct {
	// Enabled enables dynamic index routing.
	//
	// Deprecated: [v0.122.0] This config is now ignored. Dynamic index routing is always done by default.
	Enabled bool `mapstructure:"enabled"`
}

type DynamicIDSettings struct {
	Enabled bool `mapstructure:"enabled"`
}

type DynamicPipelineSettings struct {
	Enabled bool `mapstructure:"enabled"`
}

// AuthenticationSettings defines user authentication related settings.
type AuthenticationSettings struct {
	// User is used to configure HTTP Basic Authentication.
	User string `mapstructure:"user"`

	// Password is used to configure HTTP Basic Authentication.
	Password configopaque.String `mapstructure:"password"`

	// APIKey is used to configure ApiKey based Authentication.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-create-api-key.html
	APIKey configopaque.String `mapstructure:"api_key"`
}

// DiscoverySettings defines Elasticsearch node discovery related settings.
// The exporter will check Elasticsearch regularly for available nodes
// and updates the list of hosts if discovery is enabled. Newly discovered
// nodes will automatically be used for load balancing.
//
// DiscoverySettings should not be enabled when operating Elasticsearch behind a proxy
// or load balancer.
//
// https://www.elastic.co/blog/elasticsearch-sniffing-best-practices-what-when-why-how
type DiscoverySettings struct {
	// OnStart, if set, instructs the exporter to look for available Elasticsearch
	// nodes the first time the exporter connects to the cluster.
	OnStart bool `mapstructure:"on_start"`

	// Interval instructs the exporter to renew the list of Elasticsearch URLs
	// with the given interval. URLs will not be updated if Interval is <=0.
	Interval time.Duration `mapstructure:"interval"`
}

// FlushSettings defines settings for configuring the write buffer flushing
// policy in the Elasticsearch exporter. The exporter sends a bulk request with
// all events already serialized into the send-buffer.
type FlushSettings struct {
	// Bytes sets the send buffer flushing limit.
	Bytes int `mapstructure:"bytes"`

	// Interval configures the max age of a document in the send buffer.
	Interval time.Duration `mapstructure:"interval"`
}

// RetrySettings defines settings for the HTTP request retries in the Elasticsearch exporter.
// Failed sends are retried with exponential backoff.
type RetrySettings struct {
	// Enabled allows users to disable retry without having to comment out all settings.
	Enabled bool `mapstructure:"enabled"`

	// MaxRequests configures how often an HTTP request is attempted before it is assumed to be failed.
	// Deprecated: use MaxRetries instead.
	MaxRequests int `mapstructure:"max_requests"`

	// MaxRetries configures how many times an HTTP request is retried.
	MaxRetries int `mapstructure:"max_retries"`

	// InitialInterval configures the initial waiting time if a request failed.
	InitialInterval time.Duration `mapstructure:"initial_interval"`

	// MaxInterval configures the max waiting time if consecutive requests failed.
	MaxInterval time.Duration `mapstructure:"max_interval"`

	// RetryOnStatus configures the status codes that trigger request or document level retries.
	RetryOnStatus []int `mapstructure:"retry_on_status"`
}

type MappingsSettings struct {
	// Mode configures the default document mapping mode.
	//
	// The mode may be overridden in two ways:
	//  - by the client metadata key X-Elastic-Mapping-Mode, if specified
	//  - by the scope attribute elastic.mapping.mode, if specified
	//
	// The order of precedence is:
	//   scope attribute > client metadata > default mode.
	Mode string `mapstructure:"mode"`

	// AllowedModes controls the allowed document mapping modes
	// specified through X-Elastic-Mapping-Mode client metadata.
	//
	// If unspecified, all mapping modes are allowed.
	AllowedModes []string `mapstructure:"allowed_modes"`
}

type MappingMode int

// Enum values for MappingMode.
const (
	MappingNone MappingMode = iota
	MappingECS
	MappingOTel
	MappingRaw
	MappingBodyMap

	// NumMappingModes remain last, it is used for sizing arrays.
	NumMappingModes
)

func (m MappingMode) String() string {
	switch m {
	case MappingNone:
		return "none"
	case MappingECS:
		return "ecs"
	case MappingOTel:
		return "otel"
	case MappingRaw:
		return "raw"
	case MappingBodyMap:
		return "bodymap"
	}
	return ""
}

var (
	errConfigEndpointRequired = errors.New("exactly one of [endpoint, endpoints, cloudid] must be specified")
	errConfigEmptyEndpoint    = errors.New("endpoint must not be empty")
)

const defaultElasticsearchEnvName = "ELASTICSEARCH_URL"

// Validate validates the elasticsearch server configuration.
func (cfg *Config) Validate() error {
	endpoints, err := cfg.endpoints()
	if err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		if err := validateEndpoint(endpoint); err != nil {
			return fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
		}
	}

	canonicalAllowedModes := make([]string, len(cfg.Mapping.AllowedModes))
	for i, name := range cfg.Mapping.AllowedModes {
		canonicalName := canonicalMappingModeName(name)
		if _, ok := canonicalMappingModes[canonicalName]; !ok {
			return fmt.Errorf("unknown allowed mapping mode name %q", name)
		}
		canonicalAllowedModes[i] = canonicalName
	}
	if !slices.Contains(canonicalAllowedModes, canonicalMappingModeName(cfg.Mapping.Mode)) {
		return fmt.Errorf("invalid or disallowed default mapping mode %q", cfg.Mapping.Mode)
	}

	if cfg.Compression != "none" && cfg.Compression != configcompression.TypeGzip {
		return errors.New("compression must be one of [none, gzip]")
	}

	if cfg.Retry.MaxRequests != 0 && cfg.Retry.MaxRetries != 0 {
		return errors.New("must not specify both retry::max_requests and retry::max_retries")
	}
	if cfg.Retry.MaxRequests < 0 {
		return errors.New("retry::max_requests should be non-negative")
	}
	if cfg.Retry.MaxRetries < 0 {
		return errors.New("retry::max_retries should be non-negative")
	}

	if cfg.LogsIndex != "" && cfg.LogsDynamicIndex.Enabled {
		return errors.New("must not specify both logs_index and logs_dynamic_index; logs_index should be empty unless all documents should be sent to the same index")
	}
	if cfg.MetricsIndex != "" && cfg.MetricsDynamicIndex.Enabled {
		return errors.New("must not specify both metrics_index and metrics_dynamic_index; metrics_index should be empty unless all documents should be sent to the same index")
	}
	if cfg.TracesIndex != "" && cfg.TracesDynamicIndex.Enabled {
		return errors.New("must not specify both traces_index and traces_dynamic_index; traces_index should be empty unless all documents should be sent to the same index")
	}

	return nil
}

// allowedMappingModes returns a map from canonical mapping mode names to MappingModes.
func (cfg *Config) allowedMappingModes() map[string]MappingMode {
	modes := make(map[string]MappingMode)
	for _, name := range cfg.Mapping.AllowedModes {
		canonical := canonicalMappingModeName(name)
		modes[canonical] = canonicalMappingModes[canonical]
	}
	return modes
}

var canonicalMappingModes = map[string]MappingMode{
	MappingNone.String():    MappingNone,
	MappingRaw.String():     MappingRaw,
	MappingECS.String():     MappingECS,
	MappingOTel.String():    MappingOTel,
	MappingBodyMap.String(): MappingBodyMap,
}

func canonicalMappingModeName(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "", "no": // aliases for "none"
		return "none"
	default:
		return lower
	}
}

func (cfg *Config) endpoints() ([]string, error) {
	// Exactly one of endpoint, endpoints, or cloudid must be configured.
	// If none are set, then $ELASTICSEARCH_URL may be specified instead.
	var endpoints []string
	var numEndpointConfigs int
	if cfg.Endpoint != "" {
		numEndpointConfigs++
		endpoints = []string{cfg.Endpoint}
	}
	if len(cfg.Endpoints) > 0 {
		numEndpointConfigs++
		endpoints = cfg.Endpoints
	}
	if cfg.CloudID != "" {
		numEndpointConfigs++
		u, err := parseCloudID(cfg.CloudID)
		if err != nil {
			return nil, err
		}
		endpoints = []string{u.String()}
	}
	if numEndpointConfigs == 0 {
		if v := os.Getenv(defaultElasticsearchEnvName); v != "" {
			numEndpointConfigs++
			endpoints = strings.Split(v, ",")
			for i, endpoint := range endpoints {
				endpoints[i] = strings.TrimSpace(endpoint)
			}
		}
	}
	if numEndpointConfigs != 1 {
		return nil, errConfigEndpointRequired
	}
	return endpoints, nil
}

func validateEndpoint(endpoint string) error {
	if endpoint == "" {
		return errConfigEmptyEndpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf(`invalid scheme %q, expected "http" or "https"`, u.Scheme)
	}
	return nil
}

// Based on "addrFromCloudID" in go-elasticsearch.
func parseCloudID(input string) (*url.URL, error) {
	_, after, ok := strings.Cut(input, ":")
	if !ok {
		return nil, fmt.Errorf("invalid CloudID %q", input)
	}

	decoded, err := base64.StdEncoding.DecodeString(after)
	if err != nil {
		return nil, err
	}

	before, after, ok := strings.Cut(string(decoded), "$")
	if !ok {
		return nil, fmt.Errorf("invalid decoded CloudID %q", string(decoded))
	}
	return url.Parse(fmt.Sprintf("https://%s.%s", after, before))
}

func handleDeprecatedConfig(cfg *Config, logger *zap.Logger) {
	if cfg.Retry.MaxRequests != 0 {
		cfg.Retry.MaxRetries = cfg.Retry.MaxRequests - 1
		// Do not set cfg.Retry.Enabled = false if cfg.Retry.MaxRequest = 1 to avoid breaking change on behavior
		logger.Warn("retry::max_requests has been deprecated, and will be removed in a future version. Use retry::max_retries instead.")
	}
	if cfg.LogsDynamicIndex.Enabled {
		logger.Warn("logs_dynamic_index::enabled has been deprecated, and will be removed in a future version. It is now a no-op. Dynamic document routing is now the default. See Elasticsearch Exporter README.")
	}
	if cfg.MetricsDynamicIndex.Enabled {
		logger.Warn("metrics_dynamic_index::enabled has been deprecated, and will be removed in a future version. It is now a no-op. Dynamic document routing is now the default. See Elasticsearch Exporter README.")
	}
	if cfg.TracesDynamicIndex.Enabled {
		logger.Warn("traces_dynamic_index::enabled has been deprecated, and will be removed in a future version. It is now a no-op. Dynamic document routing is now the default. See Elasticsearch Exporter README.")
	}
}

func handleTelemetryConfig(cfg *Config, logger *zap.Logger) {
	if cfg.LogRequestBody {
		logger.Warn("telemetry::log_request_body is enabled, and may expose sensitive data; It should only be used for testing or debugging.")
	}
	if cfg.LogResponseBody {
		logger.Warn("telemetry::log_response_body is enabled, and may expose sensitive data; It should only be used for testing or debugging.")
	}
	if cfg.LogFailedDocsInput {
		logger.Warn("telemetry::log_failed_docs_input is enabled, and may expose sensitive data; It should only be used for testing or debugging.")
	}
}
