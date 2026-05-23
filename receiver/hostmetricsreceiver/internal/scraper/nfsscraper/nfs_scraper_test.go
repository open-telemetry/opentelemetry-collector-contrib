// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

func mockGetNfsStats() (*NfsStats, error) {
	nfsNetStats := &nfsNetStats{
		netCount:           1000,
		udpCount:           600,
		tcpCount:           400,
		tcpConnectionCount: 50,
	}

	nfsRPCStats := &nfsRPCStats{
		rpcCount:         900,
		retransmitCount:  50,
		authRefreshCount: 10,
	}

	nfsV3ProcedureStats := []callStats{
		{nfsVersion: 3, nfsCallName: "GETATTR", nfsCallCount: 100},
		{nfsVersion: 3, nfsCallName: "LOOKUP", nfsCallCount: 150},
		{nfsVersion: 3, nfsCallName: "READ", nfsCallCount: 200},
	}

	nfsV4ProcedureStats := []callStats{ // Note: As per nfs_scraper_linux.go, these are actually V4 Operations
		{nfsVersion: 4, nfsCallName: "READ", nfsCallCount: 250},
		{nfsVersion: 4, nfsCallName: "WRITE", nfsCallCount: 180},
		{nfsVersion: 4, nfsCallName: "OPEN", nfsCallCount: 120},
	}

	nfsV4OperationStats := []callStats{
		{nfsVersion: 4, nfsCallName: "ACCESS", nfsCallCount: 90},
		{nfsVersion: 4, nfsCallName: "GETATTR", nfsCallCount: 110},
		{nfsVersion: 4, nfsCallName: "COMMIT", nfsCallCount: 70},
	}

	return &NfsStats{
		nfsNetStats:         nfsNetStats,
		nfsRPCStats:         nfsRPCStats,
		nfsV3ProcedureStats: nfsV3ProcedureStats,
		nfsV4ProcedureStats: nfsV4ProcedureStats,
		nfsV4OperationStats: nfsV4OperationStats,
	}, nil
}

// for testing purposes. It initializes all nested structs and slices with sample values.
func mockGetNfsdStats() (*nfsdStats, error) {
	// Populate nfsdRepcacheStats with sample data
	repcacheStats := &nfsdRepcacheStats{
		hits:    15000,
		misses:  320,
		nocache: 150,
	}

	// Populate nfsdFhStats with sample data
	fhStats := &nfsdFhStats{
		stale: 25,
	}

	// Populate nfsdIoStats with sample data
	ioStats := &nfsdIoStats{
		read:  8000000,
		write: 4500000,
	}

	// Populate nfsdThreadStats with sample data
	threadStats := &nfsdThreadStats{
		threads: 16,
	}

	// Populate nfsdNetStats with sample data
	netStats := &nfsdNetStats{
		netCount:           25000,
		udpCount:           0,
		tcpCount:           25000,
		tcpConnectionCount: 128,
	}

	// Populate nfsdRPCStats with sample data
	rpcStats := &nfsdRPCStats{
		rpcCount:       30000,
		badCount:       15,
		badFmtCount:    5,
		badAuthCount:   5,
		badClientCount: 5,
	}

	// Populate nfsdV3ProcedureStats with a slice of sample callStats
	v3ProcStats := []callStats{
		{nfsVersion: 3, nfsCallName: "GETATTR", nfsCallCount: 4500},
		{nfsVersion: 3, nfsCallName: "SETATTR", nfsCallCount: 800},
		{nfsVersion: 3, nfsCallName: "LOOKUP", nfsCallCount: 3200},
		{nfsVersion: 3, nfsCallName: "ACCESS", nfsCallCount: 4000},
		{nfsVersion: 3, nfsCallName: "READ", nfsCallCount: 6000},
		{nfsVersion: 3, nfsCallName: "WRITE", nfsCallCount: 5500},
	}

	// Populate nfsdV4ProcedureStats with a sample callStats for COMPOUND
	// In NFSv4, most operations are wrapped in a single COMPOUND procedure.
	v4ProcStats := []callStats{
		{nfsVersion: 4, nfsCallName: "COMPOUND", nfsCallCount: 15000},
	}

	// Populate nfsdV4OperationStats with a slice of sample callStats for v4 operations
	v4OpStats := []callStats{
		{nfsVersion: 4, nfsCallName: "ACCESS", nfsCallCount: 2800},
		{nfsVersion: 4, nfsCallName: "GETATTR", nfsCallCount: 3500},
		{nfsVersion: 4, nfsCallName: "PUTFH", nfsCallCount: 3000},
		{nfsVersion: 4, nfsCallName: "READ", nfsCallCount: 2500},
		{nfsVersion: 4, nfsCallName: "WRITE", nfsCallCount: 2000},
		{nfsVersion: 4, nfsCallName: "COMMIT", nfsCallCount: 1200},
	}

	// Assemble the complete nfsdStats struct with all populated substructures
	stats := &nfsdStats{
		nfsdRepcacheStats:    repcacheStats,
		nfsdFhStats:          fhStats,
		nfsdIoStats:          ioStats,
		nfsdThreadStats:      threadStats,
		nfsdNetStats:         netStats,
		nfsdRPCStats:         rpcStats,
		nfsdV3ProcedureStats: v3ProcStats,
		nfsdV4ProcedureStats: v4ProcStats,
		nfsdV4OperationStats: v4OpStats,
	}

	return stats, nil
}

func TestScrape(t *testing.T) {
	if !supportedOS {
		t.Skip()
	}

	type testCase struct {
		name string
	}

	testCases := []testCase{
		{
			name: "All metrics",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := &nfsScraper{
				settings: scrapertest.NewNopSettings(metadata.Type),
				config: &Config{
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				},
				getNfsStats:  mockGetNfsStats,
				getNfsdStats: mockGetNfsdStats,
			}

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(t.Context())
			require.NoError(t, err)

			noAttrs := pcommon.NewMap()
			attrMap := func(m map[string]any) pcommon.Map {
				res := pcommon.NewMap()
				require.NoError(t, res.FromRaw(m))
				return res
			}
			assertMetric(t, md, "nfs.client.net.count", int64(scraper.nfsStats.nfsNetStats.udpCount), attrMap(map[string]any{"network.transport": "udp"}))
			assertMetric(t, md, "nfs.client.net.count", int64(scraper.nfsStats.nfsNetStats.tcpCount), attrMap(map[string]any{"network.transport": "tcp"}))
			assertMetric(t, md, "nfs.client.net.tcp.connection.accepted", int64(scraper.nfsStats.nfsNetStats.tcpConnectionCount), noAttrs)

			assertMetric(t, md, "nfs.client.rpc.count", int64(scraper.nfsStats.nfsRPCStats.rpcCount), noAttrs)
			assertMetric(t, md, "nfs.client.rpc.retransmit.count", int64(scraper.nfsStats.nfsRPCStats.retransmitCount), noAttrs)
			assertMetric(t, md, "nfs.client.rpc.authrefresh.count", int64(scraper.nfsStats.nfsRPCStats.authRefreshCount), noAttrs)

			for _, s := range scraper.nfsStats.nfsV3ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("onc_rpc.procedure.name", s.nfsCallName)
				assertMetric(t, md, "nfs.client.procedure.count", int64(s.nfsCallCount), attrs)
			}

			for _, s := range scraper.nfsStats.nfsV4ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("onc_rpc.procedure.name", s.nfsCallName)
				assertMetric(t, md, "nfs.client.procedure.count", int64(s.nfsCallCount), attrs)
			}

			for _, s := range scraper.nfsStats.nfsV4OperationStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("nfs.operation.name", s.nfsCallName)
				assertMetric(t, md, "nfs.client.operation.count", int64(s.nfsCallCount), attrs)
			}

			assertMetric(t, md, "nfs.server.repcache.requests", int64(scraper.nfsdStats.nfsdRepcacheStats.hits), attrMap(map[string]any{"nfs.server.repcache.status": "hit"}))
			assertMetric(t, md, "nfs.server.repcache.requests", int64(scraper.nfsdStats.nfsdRepcacheStats.misses), attrMap(map[string]any{"nfs.server.repcache.status": "miss"}))
			assertMetric(t, md, "nfs.server.repcache.requests", int64(scraper.nfsdStats.nfsdRepcacheStats.nocache), attrMap(map[string]any{"nfs.server.repcache.status": "nocache"}))

			assertMetric(t, md, "nfs.server.fh.stale.count", int64(scraper.nfsdStats.nfsdFhStats.stale), noAttrs)

			assertMetric(t, md, "nfs.server.io", int64(scraper.nfsdStats.nfsdIoStats.read), attrMap(map[string]any{"network.io.direction": "receive"}))
			assertMetric(t, md, "nfs.server.io", int64(scraper.nfsdStats.nfsdIoStats.write), attrMap(map[string]any{"network.io.direction": "transmit"}))

			assertMetric(t, md, "nfs.server.thread.count", int64(scraper.nfsdStats.nfsdThreadStats.threads), noAttrs)

			assertMetric(t, md, "nfs.server.net.count", int64(scraper.nfsdStats.nfsdNetStats.udpCount), attrMap(map[string]any{"network.transport": "udp"}))
			assertMetric(t, md, "nfs.server.net.count", int64(scraper.nfsdStats.nfsdNetStats.tcpCount), attrMap(map[string]any{"network.transport": "tcp"}))
			assertMetric(t, md, "nfs.server.net.tcp.connection.accepted", int64(scraper.nfsdStats.nfsdNetStats.tcpConnectionCount), noAttrs)

			assertMetric(t, md, "nfs.server.rpc.count", int64(scraper.nfsdStats.nfsdRPCStats.badFmtCount), attrMap(map[string]any{"error.type": "format"}))
			assertMetric(t, md, "nfs.server.rpc.count", int64(scraper.nfsdStats.nfsdRPCStats.badAuthCount), attrMap(map[string]any{"error.type": "auth"}))
			assertMetric(t, md, "nfs.server.rpc.count", int64(scraper.nfsdStats.nfsdRPCStats.badClientCount), attrMap(map[string]any{"error.type": "client"}))

			for _, s := range scraper.nfsdStats.nfsdV3ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("onc_rpc.procedure.name", s.nfsCallName)
				assertMetric(t, md, "nfs.server.procedure.count", int64(s.nfsCallCount), attrs)
			}

			for _, s := range scraper.nfsdStats.nfsdV4ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("onc_rpc.procedure.name", s.nfsCallName)
				assertMetric(t, md, "nfs.server.procedure.count", int64(s.nfsCallCount), attrs)
			}
			for _, s := range scraper.nfsdStats.nfsdV4OperationStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("onc_rpc.version", s.nfsVersion)
				attrs.PutStr("nfs.operation.name", s.nfsCallName)
				assertMetric(t, md, "nfs.server.operation.count", int64(s.nfsCallCount), attrs)
			}
		})
	}
}

func assertMetric(t *testing.T, metrics pmetric.Metrics, name string, value int64, attributes pcommon.Map) {
	metric, found := findMetric(metrics, name)
	require.True(t, found, "Metric %q not found", name)

	var dps pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	default:
		t.Fatalf("unexpected metric type %s for metric %s", metric.Type(), name)
		return
	}

	var foundDp bool
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		if dp.IntValue() == value && dp.Attributes().Equal(attributes) {
			foundDp = true
			break
		}
	}

	assert.True(t, foundDp, "Datapoint not found for metric %q with value %d and attributes %v", name, value, attributes)
}

// findMetric searches for a metric by name in the pmetric.Metrics object.
func findMetric(metrics pmetric.Metrics, name string) (pmetric.Metric, bool) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() == name {
					return m, true
				}
			}
		}
	}
	return pmetric.Metric{}, false
}
