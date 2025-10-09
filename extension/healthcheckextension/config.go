// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

type ResponseBodySettings struct {
	// Healthy represents the body of the response returned when the collector is healthy.
	// The default value is ""
	Healthy string `mapstructure:"healthy"`

	// Unhealthy represents the body of the response returned when the collector is unhealthy.
	// The default value is ""
	Unhealthy string `mapstructure:"unhealthy"`
}

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// Path represents the path the health check service will serve.
	// The default path is "/".
	Path string `mapstructure:"path"`

	// ResponseBody represents the body of the response returned by the health check service.
	// This overrides the default response that it would return.
	ResponseBody *ResponseBodySettings `mapstructure:"response_body"`

	// CheckCollectorPipeline contains the list of settings of collector pipeline health check
	CheckCollectorPipeline checkCollectorPipelineSettings `mapstructure:"check_collector_pipeline"`

	// HTTP contains the configuration for the v2 HTTP healthcheck server.
	HTTP *healthcheck.HTTPConfig `mapstructure:"http"`

	// GRPC contains the configuration for the v2 gRPC healthcheck server.
	GRPC *healthcheck.GRPCConfig `mapstructure:"grpc"`

	// ComponentHealth configuration shared between HTTP and gRPC v2 servers.
	ComponentHealth *healthcheck.ComponentHealthConfig `mapstructure:"component_health"`

	hasV2Settings bool
}

var _ component.Config = (*Config)(nil)

var (
	errNoEndpointProvided                      = errors.New("bad config: endpoint must be specified")
	errInvalidExporterFailureThresholdProvided = errors.New("bad config: exporter_failure_threshold expects a positive number")
	errInvalidPath                             = errors.New("bad config: path must start with /")
	errV2ConfigWithoutGate                     = errors.New("bad config: enabling v2 behavior requires the feature gate extension.healthcheck.disableCompatibilityWrapper")
)

// Validate checks if the extension configuration is valid.
func (cfg *Config) Validate() error {
	if disableCompatibilityWrapperGate.IsEnabled() {
		internalCfg := cfg.toInternalConfig(true)
		return internalCfg.Validate()
	}

	if cfg.hasV2Settings || cfg.HTTP != nil || cfg.GRPC != nil || cfg.ComponentHealth != nil {
		return errV2ConfigWithoutGate
	}

	return cfg.validateLegacy()
}

func (cfg *Config) validateLegacy() error {
	if _, err := time.ParseDuration(cfg.CheckCollectorPipeline.Interval); err != nil {
		return fmt.Errorf("bad config: invalid interval %q: %w", cfg.CheckCollectorPipeline.Interval, err)
	}
	if cfg.Endpoint == "" {
		return errNoEndpointProvided
	}
	if cfg.CheckCollectorPipeline.ExporterFailureThreshold <= 0 {
		return errInvalidExporterFailureThresholdProvided
	}
	if !strings.HasPrefix(cfg.Path, "/") {
		return errInvalidPath
	}
	return nil
}

// toInternalConfig builds the shared healthcheck configuration used when the feature gate is enabled.
func (cfg *Config) toInternalConfig(enableV2 bool) healthcheck.Config {
	internal := healthcheck.Config{
		LegacyConfig: healthcheck.HTTPLegacyConfig{
			ServerConfig: cfg.ServerConfig,
			Path:         cfg.Path,
			ResponseBody: convertResponseBody(cfg.ResponseBody),
			CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
				Enabled:                  cfg.CheckCollectorPipeline.Enabled,
				Interval:                 cfg.CheckCollectorPipeline.Interval,
				ExporterFailureThreshold: cfg.CheckCollectorPipeline.ExporterFailureThreshold,
			},
			UseV2: enableV2,
		},
		ComponentHealthConfig: cloneComponentHealth(cfg.ComponentHealth),
		GRPCConfig:            cloneGRPCConfig(cfg.GRPC),
		HTTPConfig:            cloneHTTPConfig(cfg.HTTP),
	}

	if internal.HTTPConfig == nil && enableV2 {
		internal.HTTPConfig = &healthcheck.HTTPConfig{
			ServerConfig: cfg.ServerConfig,
			Status: healthcheck.PathConfig{
				Enabled: true,
				Path:    cfg.Path,
			},
			Config: healthcheck.PathConfig{
				Enabled: false,
				Path:    "/config",
			},
		}
	}

	if enableV2 {
		internal.UseV2 = true
	}

	return internal
}

// Unmarshal keeps track of whether v2 configuration options were specified.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	cfg.hasV2Settings = conf.IsSet("http") || conf.IsSet("grpc") || conf.IsSet("component_health")

	type raw Config
	return conf.Unmarshal((*raw)(cfg))
}

func convertResponseBody(body *ResponseBodySettings) *healthcheck.ResponseBodyConfig {
	if body == nil {
		return nil
	}
	return &healthcheck.ResponseBodyConfig{
		Healthy:   body.Healthy,
		Unhealthy: body.Unhealthy,
	}
}

func cloneComponentHealth(cfg *healthcheck.ComponentHealthConfig) *healthcheck.ComponentHealthConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

func cloneGRPCConfig(cfg *healthcheck.GRPCConfig) *healthcheck.GRPCConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

func cloneHTTPConfig(cfg *healthcheck.HTTPConfig) *healthcheck.HTTPConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

type checkCollectorPipelineSettings struct {
	// Enabled indicates whether to not enable collector pipeline check.
	Enabled bool `mapstructure:"enabled"`
	// Interval the time range to check healthy status of collector pipeline
	Interval string `mapstructure:"interval"`
	// ExporterFailureThreshold is the threshold of exporter failure numbers during the Interval
	ExporterFailureThreshold int `mapstructure:"exporter_failure_threshold"`
}
