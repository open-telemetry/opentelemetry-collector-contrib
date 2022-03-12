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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}

	scraper := newMySQLScraper(zap.NewNop(), cfg)
	scraper.sqlclient = &mockClient{}

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))
}

var _ client = (*mockClient)(nil)

type mockClient struct{}

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
	return readFile("global_stats")
}

func (c *mockClient) getInnodbStats() (map[string]string, error) {
	return readFile("innodb_stats")
}

func (c *mockClient) Close() error {
	return nil
}
