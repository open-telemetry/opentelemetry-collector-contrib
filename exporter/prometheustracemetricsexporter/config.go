// Copyright  OpenTelemetry Authors
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

package prometheustracemetricsexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for Trace Metrics exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	ScrapePath       string `mapstructure:"scrape_path"`
	ScrapeListenAddr string `mapstructure:"scrape_listen"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.ScrapePath) == 0 {
		return fmt.Errorf("scrape_path can't be empty")
	}
	if len(cfg.ScrapeListenAddr) == 0 {
		return fmt.Errorf("scrape_listen can't be empty")
	}

	return nil
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		ScrapePath:       "/trace_metrics",
		ScrapeListenAddr: ":8000",
	}
}
