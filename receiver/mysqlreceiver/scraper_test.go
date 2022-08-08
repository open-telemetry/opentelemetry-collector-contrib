// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysqlreceiver

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScrape(t *testing.T) {
	t.Run("successful scrape", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}

		scraper := newMySQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile: "global_stats",
			innodbStatsFile: "innodb_stats",
		}

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))
	})

	t.Run("scrape has partial failure", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}

		scraper := newMySQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile: "global_stats_partial",
			innodbStatsFile: "innodb_stats_empty",
		}

		actualMetrics, scrapeErr := scraper.scrape(context.Background())
		require.Error(t, scrapeErr)

		expectedFile := filepath.Join("testdata", "scraper", "expected_partial.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		assert.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))

		var partialError scrapererror.PartialScrapeError
		require.True(t, errors.As(scrapeErr, &partialError), "returned error was not PartialScrapeError")
		// 5 comes from 4 failed "must-have" metrics that aren't present,
		// and the other failure comes from a row that fails to parse as a number
		require.Equal(t, partialError.Failed, 5, "Expected partial error count to be 5")
	})

}

var _ client = (*mockClient)(nil)

type mockClient struct {
	globalStatsFile string
	innodbStatsFile string
}

func readFile(fname string) (map[string]string, error) {
	var stats = map[string]string{}
	file, err := os.Open(filepath.Join("testdata", "scraper", fname+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.Split(scanner.Text(), "\t")
		stats[text[0]] = text[1]
	}
	return stats, nil
}

func (c *mockClient) Connect() error {
	return nil
}

func (c *mockClient) getGlobalStats() (map[string]string, error) {
	return readFile(c.globalStatsFile)
}

func (c *mockClient) getInnodbStats() (map[string]string, error) {
	return readFile(c.innodbStatsFile)
}

func (c *mockClient) Close() error {
	return nil
}
