// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	dockerReceiver "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker/receiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		dockerReceiver.CreateDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	dockerConfig := config.(*dockerReceiver.Config)
	dsr := dockerReceiver.NewMetricsReceiver(params, dockerConfig)

	scrp, err := scraperhelper.NewScraper(metadata.Type.String(), dsr.ScrapeV2, scraperhelper.WithStart(dsr.Start), scraperhelper.WithShutdown(dsr.Shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&dsr.Config.ControllerConfig, params, consumer, scraperhelper.AddScraper(scrp))
}
