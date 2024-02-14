// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_scraper_readStats(t *testing.T) {
	f, err := os.MkdirTemp("", "haproxytest")
	require.NoError(t, err)
	socketAddr := filepath.Join(f, "testhaproxy.sock")
	l, err := net.Listen("unix", socketAddr)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		c, err2 := l.Accept()
		require.NoError(t, err2)

		buf := make([]byte, 512)
		nr, err2 := c.Read(buf)
		require.NoError(t, err2)

		data := string(buf[0:nr])
		switch data {
		case "show stats\n":
			stats, err2 := os.ReadFile(filepath.Join("testdata", "stats.txt"))
			require.NoError(t, err2)
			_, err2 = c.Write(stats)
			require.NoError(t, err2)
		default:
			require.Fail(t, fmt.Sprintf("invalid message: %v", data))
		}
	}()

	haProxyCfg := newDefaultConfig().(*Config)
	haProxyCfg.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopCreateSettings())
	m, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.NotNil(t, m)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, m, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceAttributeValue("haproxy.addr"),
		pmetrictest.IgnoreResourceMetricsOrder()))
}

func Test_scraper_readStatsWithIncompleteValues(t *testing.T) {
	f, err := os.MkdirTemp("", "haproxytest")
	require.NoError(t, err)
	socketAddr := filepath.Join(f, "testhaproxy.sock")
	l, err := net.Listen("unix", socketAddr)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		c, err2 := l.Accept()
		require.NoError(t, err2)

		buf := make([]byte, 512)
		nr, err2 := c.Read(buf)
		require.NoError(t, err2)

		data := string(buf[0:nr])
		switch data {
		case "show stats\n":
			stats, err2 := os.ReadFile(filepath.Join("testdata", "30252_stats.txt"))
			require.NoError(t, err2)
			_, err2 = c.Write(stats)
			require.NoError(t, err2)
		default:
			require.Fail(t, fmt.Sprintf("invalid message: %v", data))
		}
	}()

	haProxyCfg := newDefaultConfig().(*Config)
	haProxyCfg.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopCreateSettings())
	m, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.NotNil(t, m)

	expectedFile := filepath.Join("testdata", "scraper", "30252_expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, m, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceAttributeValue("haproxy.addr"),
		pmetrictest.IgnoreResourceMetricsOrder()))
}
