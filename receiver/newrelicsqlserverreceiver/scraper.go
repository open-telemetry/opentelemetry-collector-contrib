// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/scrapers"
)

// sqlServerScraper handles SQL Server metrics collection following nri-mssql patterns
type sqlServerScraper struct {
	connection      *SQLConnection
	config          *Config
	logger          *zap.Logger
	startTime       pcommon.Timestamp
	settings        receiver.Settings
	instanceScraper *scrapers.InstanceScraper
}

// newSqlServerScraper creates a new SQL Server scraper with structured approach
func newSqlServerScraper(settings receiver.Settings, cfg *Config) *sqlServerScraper {
	return &sqlServerScraper{
		config:   cfg,
		logger:   settings.Logger,
		settings: settings,
	}
}

// start initializes the scraper and establishes database connection
func (s *sqlServerScraper) start(ctx context.Context, _ component.Host) error {
	s.logger.Info("Starting New Relic SQL Server receiver")

	connection, err := NewSQLConnection(ctx, s.config, s.logger)
	if err != nil {
		s.logger.Error("Failed to connect to SQL Server", zap.Error(err))
		return err
	}
	s.connection = connection
	s.startTime = pcommon.NewTimestampFromTime(time.Now())

	if err := s.connection.Ping(ctx); err != nil {
		s.logger.Error("Failed to ping SQL Server", zap.Error(err))
		return err
	}

	// Initialize instance scraper with structured approach
	s.instanceScraper = scrapers.NewInstanceScraper(s.connection, s.logger)

	s.logger.Info("Successfully connected to SQL Server",
		zap.String("hostname", s.config.Hostname),
		zap.String("port", s.config.Port))

	return nil
}

// shutdown closes the database connection
func (s *sqlServerScraper) shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down New Relic SQL Server receiver")
	if s.connection != nil {
		s.connection.Close()
	}
	return nil
}

// scrape collects SQL Server instance metrics using structured approach
func (s *sqlServerScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Scraping SQL Server instance metrics")

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	attrs := resourceMetrics.Resource().Attributes()
	attrs.PutStr("server.address", s.config.Hostname)
	attrs.PutStr("server.port", s.config.Port)
	attrs.PutStr("db.system", "mssql")
	attrs.PutStr("service.name", "sql-server-monitoring")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("newrelicsqlserverreceiver")

	// Use structured approach to scrape instance buffer metrics
	if err := s.instanceScraper.ScrapeInstanceBufferMetrics(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape instance buffer metrics", zap.Error(err))
		return metrics, err
	}

	return metrics, nil
}
