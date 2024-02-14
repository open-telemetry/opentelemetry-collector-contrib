// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

const (
	defaultGranularity   = "PT1M" // 1-minute, as per https://docs.atlas.mongodb.com/reference/api/process-measurements/
	defaultAlertsEnabled = false
	defaultLogsEnabled   = false
)

// NewFactory creates a factory for MongoDB Atlas receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createCombinedLogReceiver, metadata.LogsStability))

}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	recv := newMongoDBAtlasReceiver(params, cfg)
	ms, err := newMongoDBAtlasScraper(recv)
	if err != nil {
		return nil, fmt.Errorf("unable to create a MongoDB Atlas Scaper instance: %w", err)
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(ms))
}

func createCombinedLogReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)

	if !cfg.Alerts.Enabled && !cfg.Logs.Enabled && cfg.Events == nil {
		return nil, errors.New("one of 'alerts', 'events', or 'logs' must be enabled")
	}

	var err error
	recv := &combinedLogsReceiver{
		id:        params.ID,
		storageID: cfg.StorageID,
	}

	if cfg.Alerts.Enabled {
		recv.alerts, err = newAlertsReceiver(params, cfg, consumer)
		if err != nil {
			return nil, fmt.Errorf("unable to create a MongoDB Atlas Alerts Receiver instance: %w", err)
		}
	}

	if cfg.Logs.Enabled {
		recv.logs = newMongoDBAtlasLogsReceiver(params, cfg, consumer)
		// Confirm at least one project is enabled for access logs before adding
		for _, project := range cfg.Logs.Projects {
			if project.AccessLogs != nil && project.AccessLogs.IsEnabled() {
				recv.accessLogs = newAccessLogsReceiver(params, cfg, consumer)
				break
			}
		}
	}

	if cfg.Events != nil {
		recv.events = newEventsReceiver(params, cfg, consumer)
	}

	return recv, nil
}

func createDefaultConfig() component.Config {
	c := &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		Granularity:               defaultGranularity,
		BackOffConfig:             configretry.NewDefaultBackOffConfig(),
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
		Alerts: AlertConfig{
			Enabled:      defaultAlertsEnabled,
			Mode:         alertModeListen,
			PollInterval: defaultAlertsPollInterval,
			PageSize:     defaultAlertsPageSize,
			MaxPages:     defaultAlertsMaxPages,
		},
		Logs: LogConfig{
			Enabled:  defaultLogsEnabled,
			Projects: []*LogsProjectConfig{},
		},
	}
	// reset default of 1 minute to be 3 minutes in order to avoid null values for some metrics that do not publish
	// more frequently
	c.ScraperControllerSettings.CollectionInterval = 3 * time.Minute
	return c
}
