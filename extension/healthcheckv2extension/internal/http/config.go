// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"

import "go.opentelemetry.io/collector/config/confighttp"

// Config contains the v2 config for the http healthcheck service
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	Config PathConfig `mapstructure:"config"`
	Status PathConfig `mapstructure:"status"`
}

type PathConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// LegacyConfig contains the config for the original healthcheck extension. We plan to migrate
// incrementally towards the v2 config and behavior. LegacyConfig is intentionally handled
// separately here and elsewhere to facilitate its eventual removal.
type LegacyConfig struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// Path represents the path the health check service will serve.
	// The default path is "/".
	Path string `mapstructure:"path"`

	// ResponseBody represents the body of the response returned by the health check service.
	// This overrides the default response that it would return.
	ResponseBody *ResponseBodyConfig `mapstructure:"response_body"`

	// CheckCollectorPipeline contains the config for collector pipeline health checks. It is
	// retained for backwards compatibility, but is otherwise ignored.
	CheckCollectorPipeline *CheckCollectorPipelineConfig `mapstructure:"check_collector_pipeline"`

	// UseV2 is an explicit opt-in to v2 behavior. When true, LegacyConfig will be ignored.
	UseV2 bool `mapstructure:"use_v2"`
}

// ResponseBodyConfig is legacy config that is currently supported, but can eventually be
// deprecated and removed.
type ResponseBodyConfig struct {
	// Healthy represents the body of the response returned when the collector is healthy.
	// The default value is ""
	Healthy string `mapstructure:"healthy"`

	// Unhealthy represents the body of the response returned when the collector is unhealthy.
	// The default value is ""
	Unhealthy string `mapstructure:"unhealthy"`
}

// CheckCollectorPipelineConfig is legacy config that is currently ignored as the
// `check_collector_pipeline` feature in the original healtcheck extension was not working as
// expected. This is here for backwards compatibility.
type CheckCollectorPipelineConfig struct {
	// Enabled indicates whether to not enable collector pipeline check.
	Enabled bool `mapstructure:"enabled"`
	// Interval the time range to check healthy status of collector pipeline
	Interval string `mapstructure:"interval"`
	// ExporterFailureThreshold is the threshold of exporter failure numbers during the Interval
	ExporterFailureThreshold int `mapstructure:"exporter_failure_threshold"`
}
