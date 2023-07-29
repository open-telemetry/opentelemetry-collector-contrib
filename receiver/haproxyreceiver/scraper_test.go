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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
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
	require.Equal(t, 6, m.ResourceMetrics().Len())
	require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, 10, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "haproxy.bytes.input", metric.Name())
	assert.Equal(t, int64(1444), metric.Sum().DataPoints().At(0).IntValue())

}
