// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

const (
	minLogBatchInterval = 30 * time.Millisecond
)

// Config defines configuration for Logic Monitor exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	confighttp.HTTPClientSettings `mapstructure:",squash"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
	ResourceToTelemetrySettings  resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ApiToken of LM Platform
	APIToken map[string]string `mapstructure:"apitoken"`

	// Field to enable/disable batching for logs
	LogBatchingEnabled bool `mapstructure:"log_batching_enabled"`

	// Batching interval for logs
	LogBatchingInterval time.Duration `mapstructure:"log_batching_interval"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("Endpoint should not be empty")
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("Endpoint must be valid")
	}

	if c.LogBatchingInterval < minLogBatchInterval {
		return fmt.Errorf("Minimum log batching interval should be " + minLogBatchInterval.String())
	}
	return nil
}
