// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	recv, err := newMongoDBAtlasReceiver(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create a MongoDB Atlas Receiver instance: %w", err)
	}
	ms, err := newMongoDBAtlasScraper(recv)
	if err != nil {
		return nil, fmt.Errorf("unable to create a MongoDB Atlas Scraper instance: %w", err)
	}

	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, params, consumer, scraperhelper.AddScraper(metadata.Type, ms))
}

func createCombinedLogReceiver(
	_ context.Context,
	params receiver.Settings,
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
		recv.logs, err = newMongoDBAtlasLogsReceiver(params, cfg, consumer)
		if err != nil {
			return nil, fmt.Errorf("unable to create a MongoDB Atlas Logs Receiver instance: %w", err)
		}
		// Confirm at least one project is enabled for access logs before adding
		for _, project := range cfg.Logs.Projects {
			if project.AccessLogs != nil && project.AccessLogs.IsEnabled() {
				recv.accessLogs, err = newAccessLogsReceiver(params, cfg, consumer)
				if err != nil {
					return nil, fmt.Errorf("unable to create a MongoDB Atlas Access Logs Receiver instance: %w", err)
				}
				break
			}
		}
	}

	if cfg.Events != nil {
		recv.events, err = newEventsReceiver(params, cfg, consumer)
		if err != nil {
			return nil, fmt.Errorf("unable to create a MongoDB Atlas Events Receiver instance: %w", err)
		}
	}

	return recv, nil
}

func createDefaultConfig() component.Config {
	c := &Config{
		BaseURL:              mongodbatlas.CloudURL,
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		Granularity:          defaultGranularity,
		BackOffConfig:        configretry.NewDefaultBackOffConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
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
	c.CollectionInterval = 3 * time.Minute
	return c
}
