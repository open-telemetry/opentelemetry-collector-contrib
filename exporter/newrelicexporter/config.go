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
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

// Config defines configuration options for the New Relic exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// APIKey is the required authentication credentials for New Relic APIs.
	APIKey string `mapstructure:"apikey"`

	// HarvestTimeout is the total amount of time including retries that the
	// Harvester may use trying to harvest data.  By default, HarvestTimeout
	// is set to 15 seconds.
	HarvestTimeout time.Duration `mapstructure:"harvest_timeout"`

	// HarvestPeriod controls how frequently data will be sent to New Relic.
	// If HarvestPeriod is zero then NewHarvester will not spawn a goroutine
	// to send data and it is incumbent on the consumer to call
	// Harvester.HarvestNow when data should be sent. By default, HarvestPeriod
	// is set to 5 seconds.
	HarvestPeriod time.Duration `mapstructure:"harvest_period"`

	// CommonAttributes are the attributes to be applied to all telemetry
	// sent to New Relic.
	CommonAttributes map[string]interface{} `mapstructure:"common_attributes"`

	// MetricsURLOverride overrides the metrics endpoint.
	MetricsURLOverride string `mapstructure:"metrics_url_override"`

	// SpansURLOverride overrides the spans endpoint.
	SpansURLOverride string `mapstructure:"spans_url_override"`
}

func (c Config) HarvestOptions() []func(*telemetry.Config) {
	opts := []func(*telemetry.Config){
		func(cfg *telemetry.Config) {
			cfg.Product = product
			cfg.ProductVersion = version
		},
		telemetry.ConfigAPIKey(c.APIKey),
	}

	if int64(c.HarvestTimeout) != 0 {
		opts = append(opts, func(cfg *telemetry.Config) { cfg.HarvestTimeout = c.HarvestTimeout })
	}

	if int64(c.HarvestPeriod) != 0 {
		opts = append(opts, telemetry.ConfigHarvestPeriod(c.HarvestPeriod))
	}

	if len(c.CommonAttributes) > 0 {
		opts = append(opts, telemetry.ConfigCommonAttributes(c.CommonAttributes))
	}

	if c.MetricsURLOverride != "" {
		opts = append(opts, func(cfg *telemetry.Config) { cfg.MetricsURLOverride = c.MetricsURLOverride })
	}

	if c.SpansURLOverride != "" {
		opts = append(opts, func(cfg *telemetry.Config) { cfg.SpansURLOverride = c.SpansURLOverride })
	}

	return opts
}
