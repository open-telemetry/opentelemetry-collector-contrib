// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

func Test_scraper_readStats(t *testing.T) {
	f := t.TempDir()
	socketAddr := filepath.Join(f, "testhaproxy.sock")
	l, err := net.Listen("unix", socketAddr)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		c, err2 := l.Accept()
		assert.NoError(t, err2)

		buf := make([]byte, 512)
		nr, err2 := c.Read(buf)
		assert.NoError(t, err2)

		data := string(buf[0:nr])
		switch data {
		case "show stat\n":
			stats, err2 := os.ReadFile(filepath.Join("testdata", "stats.txt"))
			assert.NoError(t, err2)
			_, err2 = c.Write(stats)
			assert.NoError(t, err2)
			assert.NoError(t, c.Close())
		default:
			assert.Fail(t, fmt.Sprintf("invalid message: %v", data))
		}
	}()

	haProxyCfg := newDefaultConfig().(*Config)
	haProxyCfg.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
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
	if runtime.GOOS == "windows" {
		t.Skip("Test is failing due to t.TempDir usage on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38860")
	}
	f := t.TempDir()
	socketAddr := filepath.Join(f, "testhaproxy.sock")
	l, err := net.Listen("unix", socketAddr)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		c, err2 := l.Accept()
		assert.NoError(t, err2)

		buf := make([]byte, 512)
		nr, err2 := c.Read(buf)
		assert.NoError(t, err2)

		data := string(buf[0:nr])
		switch data {
		case "show stat\n":
			stats, err2 := os.ReadFile(filepath.Join("testdata", "30252_stats.txt"))
			assert.NoError(t, err2)
			_, err2 = c.Write(stats)
			assert.NoError(t, err2)
			assert.NoError(t, c.Close())
		default:
			assert.Fail(t, fmt.Sprintf("invalid message: %v", data))
		}
	}()

	haProxyCfg := newDefaultConfig().(*Config)
	haProxyCfg.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
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

func Test_scraper_readStatsWithNoValues(t *testing.T) {
	f := t.TempDir()
	socketAddr := filepath.Join(f, "testhaproxy.sock")
	l, err := net.Listen("unix", socketAddr)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		c, err2 := l.Accept()
		assert.NoError(t, err2)

		buf := make([]byte, 512)
		nr, err2 := c.Read(buf)
		assert.NoError(t, err2)

		data := string(buf[0:nr])
		switch data {
		case "show stat\n":
			stats, err2 := os.ReadFile(filepath.Join("testdata", "empty_stats.txt"))
			assert.NoError(t, err2)
			_, err2 = c.Write(stats)
			assert.NoError(t, err2)
			assert.NoError(t, c.Close())
		default:
			assert.Fail(t, fmt.Sprintf("invalid message: %v", data))
		}
	}()

	haProxyCfg := newDefaultConfig().(*Config)
	haProxyCfg.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
	m, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.NotNil(t, m)

	require.Equal(t, 0, m.MetricCount())
}
