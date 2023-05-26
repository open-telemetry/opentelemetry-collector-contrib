// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
	"github.com/sijms/go-ora/v2"
)

// NewFactory creates a new Oracle receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createReceiverFunc(func(dataSourceName string) (*sql.DB, error) {
			return sql.Open("oracle", dataSourceName)
		}, newDbClient), metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	cfg.CollectionInterval = 10 * time.Second

	return &Config{
		ScraperControllerSettings: cfg,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}

type sqlOpenerFunc func(dataSourceName string) (*sql.DB, error)

func createReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateMetricsFunc {
	return func(
		ctx context.Context,
		settings receiver.CreateSettings,
		cfg component.Config,
		consumer consumer.Metrics,
	) (receiver.Metrics, error) {
		sqlCfg := cfg.(*Config)
		sqlCfg.DataSource = getDataSource(*sqlCfg)
		metricsBuilder := metadata.NewMetricsBuilder(sqlCfg.MetricsBuilderConfig, settings)

		instanceName, err := getInstanceName(sqlCfg.DataSource)
		if err != nil {
			return nil, err
		}

		mp, err := newScraper(settings.ID, metricsBuilder, sqlCfg.MetricsBuilderConfig, sqlCfg.ScraperControllerSettings, settings.TelemetrySettings.Logger, func() (*sql.DB, error) {
			return sqlOpenerFunc(sqlCfg.DataSource)
		}, clientProviderFunc, instanceName)
		if err != nil {
			return nil, err
		}
		opt := scraperhelper.AddScraper(mp)

		return scraperhelper.NewScraperControllerReceiver(
			&sqlCfg.ScraperControllerSettings,
			settings,
			consumer,
			opt,
		)
	}
}

func getDataSource(cfg Config) string {
	if cfg.DataSource != "" {
		return cfg.DataSource
	}

	// Don't need to worry about errors here as config validation already checked.
	host, portStr, _ := net.SplitHostPort(cfg.Endpoint)
	port, _ := strconv.ParseInt(portStr, 10, 32)

	return go_ora.BuildUrl(host, int(port), cfg.Service, cfg.Username, cfg.Password, nil)
}

func getInstanceName(datasource string) (string, error) {
	datasourceURL, err := url.Parse(datasource)
	if err != nil {
		return "", err
	}

	instanceName := datasourceURL.Host + datasourceURL.Path
	return instanceName, nil
}
