// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package prometheusreceiver // Copyright The OpenTelemetry Authors

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const kongPort = "18001"

func TestKongIntegration(t *testing.T) {
	mockServer := setupMockKongServer(t)
	defer mockServer.Close()

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				cm, err := confmaptest.LoadConf(filepath.Join("testdata", "kong", "config.yaml"))
				require.NoError(t, err)
				require.NoError(t, cm.Unmarshal(cfg))
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "kong", "expected.yaml")),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricAttributeValue("node_id"),
			pmetrictest.IgnoreMetricAttributeValue("pid"),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreSubsequentDataPoints(),
			pmetrictest.IgnoreSummaryDataPointValueAtQuantileSliceOrder(),
			pmetrictest.IgnoreMetricAttributeValue("version", "kong_node_info"),
		),
	).Run(t)
}

func setupMockKongServer(t *testing.T) *httptest.Server {
	fullPath := filepath.Join("testdata", "kong", "scraped-kong-data.txt")
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err = w.Write(data)
		assert.NoError(t, err)
	}))

	l, _ := net.Listen("tcp", "localhost:"+kongPort)
	server.Listener = l
	server.Start()

	return server
}
