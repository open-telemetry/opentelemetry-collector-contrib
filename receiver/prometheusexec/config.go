// Copyright 2020, OpenTelemetry Authors
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

package prometheusexec

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager/config"

	"go.opentelemetry.io/collector/config/configmodels"
)

// Config definition for prometheus_exec configuration
type Config struct {
	// Generic receiver config
	configmodels.ReceiverSettings `mapstructure:",squash"`
	// ScrapeConfigs is the list of scrape configurations
	ScrapeConfigs []scrapeConfig `mapstructure:"scrape_configs"`
}

// scrapeConfig holds all the information for the subprocess manager and Prometheus receiver
type scrapeConfig struct {
	// ScrapeInterval is the time between each scrape completed by the Receiver
	ScrapeInterval time.Duration `mapstructure:"scrape_interval,omitempty"`
	// SubprocessConfigs is the list of subprocess configurations
	SubprocessConfig config.SubprocessConfig `mapstructure:",squash"`
}
