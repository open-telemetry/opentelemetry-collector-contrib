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

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

// NewFactory creates a new filestats receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithMetrics(newReceiver, metadata.MetricsStability))
}

func newDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Minute,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func newReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	fileStatsConfig := cfg.(*Config)
	metricsBuilder := metadata.NewMetricsBuilder(fileStatsConfig.MetricsBuilderConfig, settings)

	mp := newScraper(metricsBuilder, fileStatsConfig, settings.TelemetrySettings.Logger)
	s, err := scraperhelper.NewScraper(settings.ID.Name(), mp.scrape)
	if err != nil {
		return nil, err
	}
	opt := scraperhelper.AddScraper(s)

	return scraperhelper.NewScraperControllerReceiver(
		&fileStatsConfig.ScraperControllerSettings,
		settings,
		consumer,
		opt,
	)
}
