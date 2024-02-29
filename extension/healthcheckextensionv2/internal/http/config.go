// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import "go.opentelemetry.io/collector/config/confighttp"

// Settings contains the v2 settings for the http healthcheck service
type Settings struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`

	Config PathSettings `mapstructure:"config"`
	Status PathSettings `mapstructure:"status"`
}

type PathSettings struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// LegacySettings contain the settings for the original healthcheck extension. We plan to migrate
// incrementally towards the v2 settings and behavior. LegacySettings are intentionally handled
// separately here and elsewhere to facilitate their eventual removal.
type LegacySettings struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`

	// Path represents the path the health check service will serve.
	// The default path is "/".
	Path string `mapstructure:"path"`

	// ResponseBody represents the body of the response returned by the health check service.
	// This overrides the default response that it would return.
	ResponseBody *ResponseBodySettings `mapstructure:"response_body"`

	// CheckCollectorPipeline contains the list of settings of collector pipeline health check
	CheckCollectorPipeline *CheckCollectorPipelineSettings `mapstructure:"check_collector_pipeline"`

	// UseV2Settings is an explicit opt-in to v2 behavior. When true, LegacySettings will be ignored.
	UseV2Settings bool `mapstructure:"use_v2_settings"`
}

// ResponseBodySettings are legacy settings that are currently supported, but can eventually be
// deprecated and removed.
type ResponseBodySettings struct {
	// Healthy represents the body of the response returned when the collector is healthy.
	// The default value is ""
	Healthy string `mapstructure:"healthy"`

	// Unhealthy represents the body of the response returned when the collector is unhealthy.
	// The default value is ""
	Unhealthy string `mapstructure:"unhealthy"`
}

// CheckCollectorPipelineSettings are legacy settings that are currently ignored as the
// `check_collector_pipeline` feature in the original healtcheck extension was not working as
// expected. These are here for backwards compatibility.
type CheckCollectorPipelineSettings struct {
	// Enabled indicates whether to not enable collector pipeline check.
	Enabled bool `mapstructure:"enabled"`
	// Interval the time range to check healthy status of collector pipeline
	Interval string `mapstructure:"interval"`
	// ExporterFailureThreshold is the threshold of exporter failure numbers during the Interval
	ExporterFailureThreshold int `mapstructure:"exporter_failure_threshold"`
}
