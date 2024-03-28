// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 10 * time.Second
	scs.Timeout = 5 * time.Second
	return &Config{
		Endpoint:             "unix:///var/run/docker.sock",
		DockerAPIVersion:     defaultDockerAPIVersion,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Stats: StatsConfig{
			ControllerConfig: scs,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	dockerConfig := config.(*Config)
	dsr := newMetricsReceiver(params, dockerConfig)

	scrp, err := scraperhelper.NewScraper(metadata.Type.String(), dsr.scrapeV2, scraperhelper.WithStart(dsr.start), scraperhelper.WithShutdown(dsr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&dsr.config.Stats.ControllerConfig, params, consumer, scraperhelper.AddScraper(scrp))
}
