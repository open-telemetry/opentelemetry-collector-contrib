// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"runtime"
	"strings"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	corelog "github.com/DataDog/datadog-agent/comp/core/log/def"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	pkgconfigsetup "github.com/DataDog/datadog-agent/pkg/config/setup"
	pkgconfigutils "github.com/DataDog/datadog-agent/pkg/config/utils"
	"github.com/DataDog/datadog-agent/pkg/config/viperconfig"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	zlib "github.com/DataDog/datadog-agent/pkg/util/compression/impl-zlib"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/net/http/httpproxy"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// ConfigOption is a function that configures the Datadog agent config component.
// This allows for flexible configuration by different modules.
type ConfigOption func(pkgconfigmodel.Config)

// SerializerWithForwarder is an interface that extends the MetricSerializer interface
// with ability to interact directly with the underlying forwarder's lifecycle methods.
type SerializerWithForwarder interface {
	serializer.MetricSerializer
	Start() error
	State() uint32
	Stop()
}

// forwarderWithLifecycle extends the defaultforwarder.Forwarder interface
// with lifecycle management methods
type forwarderWithLifecycle interface {
	defaultforwarder.Forwarder
	Start() error
	State() uint32
	Stop()
}

// Compile-time check to ensure DefaultForwarder implements ForwarderWithLifecycle
var _ forwarderWithLifecycle = (*defaultforwarder.DefaultForwarder)(nil)

// Compile-time check to ensure datadogSerializer implements SerializerWithForwarder
var _ SerializerWithForwarder = (*datadogSerializer)(nil)

// datadogSerializer is a concrete implementation of SerializerWithForwarder that wraps
// a MetricSerializer and provides access to the underlying forwarder's lifecycle methods
type datadogSerializer struct {
	serializer.MetricSerializer
	forwarder forwarderWithLifecycle
}

// Start delegates to the underlying forwarder's Start method
func (ds *datadogSerializer) Start() error {
	return ds.forwarder.Start()
}

// State delegates to the underlying forwarder's State method
func (ds *datadogSerializer) State() uint32 {
	return ds.forwarder.State()
}

// Stop delegates to the underlying forwarder's Stop method
func (ds *datadogSerializer) Stop() {
	ds.forwarder.Stop()
}

// NewLogComponent creates a new log component for collector that uses the provided telemetry settings.
func NewLogComponent(set component.TelemetrySettings) corelog.Component {
	zlog := &ZapLogger{
		Logger: set.Logger,
	}
	return zlog
}

// NewSerializerComponent creates a new serializer that serializes and compresses payloads prior to being forwarded
func NewSerializerComponent(cfg coreconfig.Component, logger corelog.Component, hostname string) SerializerWithForwarder {
	forwarder := newForwarderComponent(cfg, logger)
	compressor := zlib.New()
	metricSerializer := serializer.NewSerializer(forwarder, nil, compressor, cfg, logger, hostname)

	return &datadogSerializer{
		MetricSerializer: metricSerializer,
		forwarder:        forwarder,
	}
}

// NewConfigComponent creates a new Datadog agent config component with the given options.
// This function uses the options pattern to allow different modules to configure
// the component with their specific needs.
func NewConfigComponent(options ...ConfigOption) coreconfig.Component {
	pkgconfig := viperconfig.NewConfig("DD", "DD", strings.NewReplacer(".", "_"))

	// Apply all configuration options
	for _, opt := range options {
		opt(pkgconfig)
	}

	return pkgconfig
}

// WithAPIConfig configures API-related settings
func WithAPIConfig(cfg *datadogconfig.Config) ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("api_key", string(cfg.API.Key), pkgconfigmodel.SourceFile)
		pkgconfig.Set("site", cfg.API.Site, pkgconfigmodel.SourceFile)
	}
}

// WithForwarderConfig configures forwarder-related settings
func WithForwarderConfig() ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("forwarder_apikey_validation_interval", 60, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_num_workers", 1, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_backoff_factor", 2, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_backoff_base", 2, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_backoff_max", 64, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_recovery_interval", 2, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("forwarder_http_protocol", "auto", pkgconfigmodel.SourceDefault)
	}
}

// WithLogsEnabled enables logs for agent config
func WithLogsEnabled() ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("logs_enabled", true, pkgconfigmodel.SourceDefault)
	}
}

// WithLogsConfig configures logs-related settings (requires WithLogsEnabled)
func WithLogsConfig(cfg *datadogconfig.Config) ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("logs_config.batch_wait", cfg.Logs.BatchWait, pkgconfigmodel.SourceFile)
		pkgconfig.Set("logs_config.use_compression", cfg.Logs.UseCompression, pkgconfigmodel.SourceFile)
		pkgconfig.Set("logs_config.compression_level", cfg.Logs.CompressionLevel, pkgconfigmodel.SourceFile)
		pkgconfig.Set("logs_config.logs_dd_url", cfg.Logs.Endpoint, pkgconfigmodel.SourceFile)
	}
}

// WithLogsDefaults configures logs default settings (requires WithLogsEnabled)
func WithLogsDefaults() ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("logs_config.auditor_ttl", pkgconfigsetup.DefaultAuditorTTL, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.batch_max_content_size", pkgconfigsetup.DefaultBatchMaxContentSize, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.batch_max_size", pkgconfigsetup.DefaultBatchMaxSize, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.force_use_http", true, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.input_chan_size", pkgconfigsetup.DefaultInputChanSize, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.max_message_size_bytes", pkgconfigsetup.DefaultMaxMessageSizeBytes, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.run_path", "/opt/datadog-agent/run", pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.sender_backoff_factor", pkgconfigsetup.DefaultLogsSenderBackoffFactor, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.sender_backoff_base", pkgconfigsetup.DefaultLogsSenderBackoffBase, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.sender_backoff_max", pkgconfigsetup.DefaultLogsSenderBackoffMax, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.sender_recovery_interval", pkgconfigsetup.DefaultForwarderRecoveryInterval, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.stop_grace_period", 30, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("logs_config.use_v2_api", true, pkgconfigmodel.SourceDefault)
		pkgconfig.SetKnown("logs_config.dev_mode_no_ssl")
		// add logs config pipelines config value, see https://github.com/DataDog/datadog-agent/pull/31190
		logsPipelines := min(4, runtime.GOMAXPROCS(0))
		pkgconfig.Set("logs_config.pipelines", logsPipelines, pkgconfigmodel.SourceDefault)
	}
}

// WithLogLevel configures log level settings (requires WithLogsEnabled)
func WithLogLevel(set component.TelemetrySettings) ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("log_level", set.Logger.Level().String(), pkgconfigmodel.SourceFile)
	}
}

// WithPayloadsConfig configures payload settings
func WithPayloadsConfig() ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("enable_payloads.events", true, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("enable_payloads.json_to_v1_intake", true, pkgconfigmodel.SourceDefault)
		pkgconfig.Set("enable_sketch_stream_payload_serialization", true, pkgconfigmodel.SourceDefault)
	}
}

// WithProxyFromEnv configures proxy settings from environment variables
func WithProxyFromEnv() ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		setProxyFromEnv(pkgconfig)
	}
}

// WithCustomConfig allows setting arbitrary configuration values
func WithCustomConfig(key string, value any, source pkgconfigmodel.Source) ConfigOption {
	return func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set(key, value, source)
	}
}

func setProxyFromEnv(config pkgconfigmodel.Config) {
	proxyConfig := httpproxy.FromEnvironment()
	config.Set("proxy.http", proxyConfig.HTTPProxy, pkgconfigmodel.SourceEnvVar)
	config.Set("proxy.https", proxyConfig.HTTPSProxy, pkgconfigmodel.SourceEnvVar)

	// If this is set to an empty []string, viper will have a type conflict when merging
	// this config during secrets resolution. It unmarshals empty yaml lists to type
	// []any, which will then conflict with type []string and fail to merge.
	var noProxy []any
	for _, v := range strings.Split(proxyConfig.NoProxy, ",") {
		noProxy = append(noProxy, v)
	}
	config.Set("proxy.no_proxy", noProxy, pkgconfigmodel.SourceEnvVar)
}

// newForwarderComponent creates a new forwarder that sends payloads to Datadog backend
func newForwarderComponent(cfg coreconfig.Component, log corelog.Component) forwarderWithLifecycle {
	keysPerDomain := map[string][]pkgconfigutils.APIKeys{
		"https://api." + cfg.GetString("site"): {pkgconfigutils.NewAPIKeys("api_key", cfg.GetString("api_key"))},
	}
	forwarderOptions := defaultforwarder.NewOptions(cfg, log, keysPerDomain)
	forwarderOptions.DisableAPIKeyChecking = true
	return defaultforwarder.NewDefaultForwarder(cfg, log, forwarderOptions)
}
