// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"time"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration options for the New Relic exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// APIKey is the required authentication credentials for New Relic APIs. This field specifies the default key.
	APIKey string `mapstructure:"apikey"`

	// APIKeyHeader may be specified to instruct the exporter to extract the API key from the request context.
	APIKeyHeader string `mapstructure:"api_key_header"`

	// Timeout is the total amount of time spent attempting a request,
	// including retries, before abandoning and dropping data. Default is 15
	// seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// CommonAttributes are the attributes to be applied to all telemetry
	// sent to New Relic.
	CommonAttributes map[string]interface{} `mapstructure:"common_attributes"`

	// MetricsHostOverride overrides the metrics endpoint.
	MetricsHostOverride string `mapstructure:"metrics_host_override"`

	// SpansHostOverride overrides the spans endpoint.
	SpansHostOverride string `mapstructure:"spans_host_override"`

	// MetricsInsecure disables TLS on the metrics endpoint.
	metricsInsecure bool

	// SpansInsecure disables TLS on the spans endpoint.
	spansInsecure bool
}

// HarvestOption sets all relevant Config values when instantiating a New
// Relic Harvester.
func (c Config) HarvestOption(cfg *telemetry.Config) {
	cfg.APIKey = c.APIKey
	cfg.HarvestPeriod = 0 // use collector harvest period.
	cfg.HarvestTimeout = c.Timeout
	cfg.CommonAttributes = c.CommonAttributes
	cfg.Product = product
	cfg.ProductVersion = version
	var prefix string
	if c.metricsInsecure {
		prefix = "http://"
	} else {
		prefix = "https://"
	}
	cfg.MetricsURLOverride = prefix + c.MetricsHostOverride
}
