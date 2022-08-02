// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	config.ExtensionSettings      `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	confighttp.HTTPServerSettings `mapstructure:",squash"`

	// Path represents the path the health check service will serve.
	// The default path is "/".
	Path string `mapstructure:"path"`

	// CheckCollectorPipeline contains the list of settings of collector pipeline health check
	CheckCollectorPipeline checkCollectorPipelineSettings `mapstructure:"check_collector_pipeline"`
}

var _ config.Extension = (*Config)(nil)
var (
	errNoEndpointProvided                      = errors.New("bad config: endpoint must be specified")
	errInvalidExporterFailureThresholdProvided = errors.New("bad config: exporter_failure_threshold expects a positive number")
	errInvalidPath                             = errors.New("bad config: path must start with /")
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	_, err := time.ParseDuration(cfg.CheckCollectorPipeline.Interval)
	if err != nil {
		return err
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

type checkCollectorPipelineSettings struct {
	// Enabled indicates whether to not enable collector pipeline check.
	Enabled bool `mapstructure:"enabled"`
	// Interval the time range to check healthy status of collector pipeline
	Interval string `mapstructure:"interval"`
	// ExporterFailureThreshold is the threshold of exporter failure numbers during the Interval
	ExporterFailureThreshold int `mapstructure:"exporter_failure_threshold"`
}
