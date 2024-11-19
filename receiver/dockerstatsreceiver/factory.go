// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
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
		ControllerConfig: scs,
		Config: docker.Config{
			Endpoint:         "unix:///var/run/docker.sock",
			DockerAPIVersion: defaultDockerAPIVersion,
			Timeout:          scs.Timeout,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	dockerConfig := config.(*Config)
	dsr := newMetricsReceiver(params, dockerConfig)

	scrp, err := scraperhelper.NewScraperWithoutType(dsr.scrapeV2, scraperhelper.WithStart(dsr.start), scraperhelper.WithShutdown(dsr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&dsr.config.ControllerConfig, params, consumer, scraperhelper.AddScraperWithType(metadata.Type, scrp))
}
