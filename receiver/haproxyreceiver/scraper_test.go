// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

func Test_scraper_readStats(t *testing.T) {
	l, socketAddr := listenUnix(t)
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
	haProxyCfg.ClientConfig.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
	m, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.NotNil(t, m)

	expectedFile := filepath.Join("testdata", "scraper", "metrics.assert.yaml")
	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, m))

	require.NoError(t, pmetricassert.AssertMetrics(expectedFile, m))
}

func Test_scraper_readStatsWithIncompleteValues(t *testing.T) {
	l, socketAddr := listenUnix(t)
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
	haProxyCfg.ClientConfig.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
	m, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.NotNil(t, m)

	expectedFile := filepath.Join("testdata", "scraper", "30252_metrics.assert.yaml")
	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, m))

	require.NoError(t, pmetricassert.AssertMetrics(expectedFile, m))
}

func Test_scraper_readStatsWithNoValues(t *testing.T) {
	l, socketAddr := listenUnix(t)
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
	haProxyCfg.ClientConfig.Endpoint = socketAddr
	s := newScraper(haProxyCfg, receivertest.NewNopSettings(metadata.Type))
	m, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.NotNil(t, m)

	require.Equal(t, 0, m.MetricCount())
}

func listenUnix(tb testing.TB) (net.Listener, string) {
	// Note that we intentionally do not use tb.TempDir() here, as we need to
	// create a path that is as short as possible. This is based on code from
	// Go's net package.
	tempdir, err := os.MkdirTemp("", "") //nolint:usetesting
	require.NoError(tb, err)
	tb.Cleanup(func() {
		assert.NoError(tb, os.RemoveAll(tempdir))
	})
	l, err := net.Listen("unix", filepath.Join(tempdir, "sock"))
	require.NoError(tb, err)
	tb.Cleanup(func() { assert.NoError(tb, l.Close()) })
	return l, l.Addr().String()
}
