// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestEmptyScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true

	assert.NoError(t, cfg.Validate())

	// Ensure there aren't any scrapers when all metrics are disabled.
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIoReadLatency.Enabled = false
	scrapers := setupSQLServerScrapers(receivertest.NewNopCreateSettings(), cfg)
	assert.Empty(t, scrapers)
}

func TestSuccessfulScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	// Ensure all metrics are received when all are enabled.
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIoReadLatency.Enabled = true
	scrapers := setupSQLServerScrapers(receivertest.NewNopCreateSettings(), cfg)
	assert.NotNil(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer func() { assert.NoError(t, scraper.Shutdown(context.Background())) }()

		scraper.client = mockClient{
			instanceName: scraper.instanceName,
			SQL:          scraper.sqlQuery,
		}

		actualMetrics, err := scraper.Scrape(context.Background())
		assert.NoError(t, err)

		expectedFile := filepath.Join("testdata", "expected_database_io.yaml")
		// Uncomment line below to re-generate expected metrics.
		// golden.WriteMetrics(t, expectedFile, actualMetrics)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		assert.NoError(t, err)

		assert.NoError(t, pmetrictest.CompareMetrics(actualMetrics, expectedMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()))
	}
}

func TestScrapeInvalidQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true

	assert.NoError(t, cfg.Validate())

	// Ensure all metrics are received when all are enabled.
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIoReadLatency.Enabled = true
	scrapers := setupSQLServerScrapers(receivertest.NewNopCreateSettings(), cfg)
	assert.NotNil(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer func() { assert.NoError(t, scraper.Shutdown(context.Background())) }()

		scraper.client = mockClient{
			instanceName: scraper.instanceName,
			SQL:          "Invalid SQL query",
		}

		actualMetrics, err := scraper.Scrape(context.Background())
		assert.Error(t, err)
		assert.Empty(t, actualMetrics)
	}
}

var _ sqlquery.DbClient = (*mockClient)(nil)

type mockClient struct {
	SQL          string
	instanceName string
}

func readFile(fname string) ([]sqlquery.StringMap, error) {
	file, err := os.ReadFile(filepath.Join("testdata", fname))
	if err != nil {
		return nil, err
	}

	var metrics []sqlquery.StringMap
	err = json.Unmarshal(file, &metrics)
	if err != nil {
		return nil, err
	}

	return metrics, nil

}

func (mc mockClient) QueryRows(context.Context, ...any) ([]sqlquery.StringMap, error) {
	if mc.SQL == getSQLServerDatabaseIOQuery(mc.instanceName) {
		queryResults, err := readFile("database_io_scraped_data.txt")
		if err != nil {
			return nil, err
		}
		return queryResults, nil
	}

	return nil, fmt.Errorf("No valid query found")
}
