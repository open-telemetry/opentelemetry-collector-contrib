// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 10 * time.Second
	scs.Timeout = 5 * time.Second
	config := *docker.NewDefaultConfig()
	config.DockerAPIVersion = defaultDockerAPIVersion
	config.Timeout = scs.Timeout
	return &Config{
		ControllerConfig:     scs,
		Config:               config,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	dockerConfig := config.(*Config)

	if err := applyContainerConventionsGates(params.Logger, &dockerConfig.MetricsBuilderConfig.Metrics); err != nil {
		return nil, err
	}

	dsr := newMetricsReceiver(params, dockerConfig)

	scrp, err := scraper.NewMetrics(dsr.scrapeV2, scraper.WithStart(dsr.start), scraper.WithShutdown(dsr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&dsr.config.ControllerConfig, params, consumer, scraperhelper.AddMetricsScraper(metadata.Type, scrp))
}

// metricReplacement pairs a legacy metric with the semantic convention metric
// that replaces it. Several legacy metrics can share one replacement.
type metricReplacement struct {
	legacy      *bool
	replacement *bool
}

// applyContainerConventionsGates enables and disables metrics according to the
// EmitV1ContainerConventions and DontEmitV0ContainerConventions feature gates.
// container.memory.total_pgfault is migrated by the generated metrics builder
// and is not handled here.
func applyContainerConventionsGates(logger *zap.Logger, metrics *metadata.MetricsConfig) error {
	emitV1 := metadata.ReceiverDockerStatsEmitV1ContainerConventionsFeatureGate.IsEnabled()
	dontEmitV0 := metadata.ReceiverDockerStatsDontEmitV0ContainerConventionsFeatureGate.IsEnabled()
	if dontEmitV0 && !emitV1 {
		return errors.New("feature gate " + metadata.ReceiverDockerStatsDontEmitV0ContainerConventionsFeatureGate.ID() +
			" requires " + metadata.ReceiverDockerStatsEmitV1ContainerConventionsFeatureGate.ID() + " to be enabled")
	}

	replacements := []metricReplacement{
		{&metrics.ContainerCPUUsageTotal.Enabled, &metrics.ContainerCPUTime.Enabled},
		{&metrics.ContainerCPUUsageSystem.Enabled, &metrics.ContainerCPUTime.Enabled},
		{&metrics.ContainerCPUUsageKernelmode.Enabled, &metrics.ContainerCPUTime.Enabled},
		{&metrics.ContainerCPUUsageUsermode.Enabled, &metrics.ContainerCPUTime.Enabled},
		{&metrics.ContainerCPUUtilization.Enabled, &metrics.ContainerCPUUsage.Enabled},
		{&metrics.ContainerMemoryUsageTotal.Enabled, &metrics.ContainerMemoryUsage.Enabled},
		{&metrics.ContainerNetworkIoUsageRxBytes.Enabled, &metrics.ContainerNetworkIo.Enabled},
		{&metrics.ContainerNetworkIoUsageTxBytes.Enabled, &metrics.ContainerNetworkIo.Enabled},
	}
	additions := []*bool{
		&metrics.ContainerMemoryAvailable.Enabled,
		&metrics.ContainerMemoryWorkingSet.Enabled,
		&metrics.ContainerDiskIo.Enabled,
	}

	if !emitV1 {
		logger.Warn("Container metrics will change to follow the semantic conventions. Enable the " +
			metadata.ReceiverDockerStatsEmitV1ContainerConventionsFeatureGate.ID() + " and " +
			metadata.ReceiverDockerStatsDontEmitV0ContainerConventionsFeatureGate.ID() + " feature gates to switch now.")
		for _, r := range replacements {
			*r.replacement = false
		}
		for _, enabled := range additions {
			*enabled = false
		}
		return nil
	}

	for _, r := range replacements {
		if *r.legacy {
			*r.replacement = true
		}
	}
	for _, enabled := range additions {
		*enabled = true
	}
	if dontEmitV0 {
		for _, r := range replacements {
			*r.legacy = false
		}
	}
	return nil
}
