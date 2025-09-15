// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/internal/metadata"
)

// NewFactory creates a factory for New Relic SQL Server receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return DefaultConfig()
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)

	// Create the main SQL Server scraper
	sqlServerScraper, err := newSQLServerScraper(params, cfg)
	if err != nil {
		return nil, err
	}

	// Create scraper with lifecycle methods
	s, err := scraper.NewMetrics(
		sqlServerScraper.scrape,
		scraper.WithStart(sqlServerScraper.start),
		scraper.WithShutdown(sqlServerScraper.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}

// sqlServerScraper is the main scraper that orchestrates all SQL Server metric collection
type sqlServerScraper struct {
	connection         *SQLConnection
	coreMetricsScraper *coreMetricsScraper
	logger             *zap.Logger
	config             *Config
}

// newSQLServerScraper creates a new SQL Server scraper instance
func newSQLServerScraper(params receiver.Settings, config *Config) (*sqlServerScraper, error) {
	logger := params.Logger

	// Create SQL connection - we'll initialize it in start() method
	var connection *SQLConnection

	// Create core metrics scraper
	coreMetricsScraper := newCoreMetricsScraper(connection, config, logger)

	return &sqlServerScraper{
		connection:         connection,
		coreMetricsScraper: coreMetricsScraper,
		logger:             logger,
		config:             config,
	}, nil
}

// start initializes the scraper and establishes database connections
func (s *sqlServerScraper) start(ctx context.Context, _ component.Host) error {
	s.logger.Info("Starting New Relic SQL Server receiver")

	// Create and test the connection
	connection, err := NewSQLConnection(ctx, s.config, s.logger)
	if err != nil {
		s.logger.Error("Failed to connect to SQL Server", zap.Error(err))
		return err
	}
	s.connection = connection

	// Test the connection with ping
	if err := s.connection.Ping(ctx); err != nil {
		s.logger.Error("Failed to ping SQL Server", zap.Error(err))
		return err
	}

	s.logger.Info("Successfully connected to SQL Server",
		zap.String("hostname", s.config.Hostname),
		zap.String("port", s.config.Port),
		zap.String("instance", s.config.Instance))

	return nil
}

// shutdown closes the database connection and cleans up resources
func (s *sqlServerScraper) shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down New Relic SQL Server receiver")

	if s.connection != nil {
		s.connection.Close()
	}

	return nil
}

// scrape collects metrics from SQL Server
func (s *sqlServerScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Scraping SQL Server metrics")

	// Create metrics container
	metrics := pmetric.NewMetrics()

	// Collect core metrics
	coreMetrics, err := s.coreMetricsScraper.scrape(ctx)
	if err != nil {
		s.logger.Error("Failed to collect core metrics", zap.Error(err))
		return metrics, err
	}

	// Merge core metrics
	coreMetrics.ResourceMetrics().MoveAndAppendTo(metrics.ResourceMetrics())

	// Note: Query performance monitoring metrics are handled by a separate scraper in the controller
	// The scraperhelper.NewMetricsController will handle multiple scrapers if we add them

	s.logger.Debug("Successfully scraped SQL Server metrics",
		zap.Int("resource_metrics_count", metrics.ResourceMetrics().Len()))

	return metrics, nil
}
