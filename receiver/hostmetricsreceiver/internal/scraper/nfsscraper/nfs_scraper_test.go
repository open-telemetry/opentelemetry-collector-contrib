// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"
)

func mockGetNfsStats() (*NfsStats, error) {
	nfsNetStats := &NfsNetStats{
		NetCount:           1000,
		UDPCount:           600,
		TCPCount:           400,
		TCPConnectionCount: 50,
	}

	nfsRPCStats := &NfsRPCStats{
		RPCCount:         900,
		RetransmitCount:  50,
		AuthRefreshCount: 10,
	}

	nfsV3ProcedureStats := &[]CallStats{
		{NFSVersion: 3, NFSCallName: "GETATTR", NFSCallCount: 100},
		{NFSVersion: 3, NFSCallName: "LOOKUP", NFSCallCount: 150},
		{NFSVersion: 3, NFSCallName: "READ", NFSCallCount: 200},
	}

	nfsV4ProcedureStats := &[]CallStats{ // Note: As per nfs_scraper_linux.go, these are actually V4 Operations
		{NFSVersion: 4, NFSCallName: "READ", NFSCallCount: 250},
		{NFSVersion: 4, NFSCallName: "WRITE", NFSCallCount: 180},
		{NFSVersion: 4, NFSCallName: "OPEN", NFSCallCount: 120},
	}

	nfsV4OperationStats := &[]CallStats{
		{NFSVersion: 4, NFSCallName: "ACCESS", NFSCallCount: 90},
		{NFSVersion: 4, NFSCallName: "GETATTR", NFSCallCount: 110},
		{NFSVersion: 4, NFSCallName: "COMMIT", NFSCallCount: 70},
	}

	return &NfsStats{
		NfsNetStats:         nfsNetStats,
		NfsRPCStats:         nfsRPCStats,
		NfsV3ProcedureStats: nfsV3ProcedureStats,
		NfsV4ProcedureStats: nfsV4ProcedureStats,
		NfsV4OperationStats: nfsV4OperationStats,
	}, nil
}

// for testing purposes. It initializes all nested structs and slices with sample values.
func mockGetNfsdStats() (*NfsdStats, error) {
	// Populate NfsdRepcacheStats with sample data
	repcacheStats := &NfsdRepcacheStats{
		Hits:    15000,
		Misses:  320,
		Nocache: 150,
	}

	// Populate NfsdFhStats with sample data
	fhStats := &NfsdFhStats{
		Stale: 25,
	}

	// Populate NfsdIoStats with sample data
	ioStats := &NfsdIoStats{
		Read:  8000000,
		Write: 4500000,
	}

	// Populate NfsdThreadStats with sample data
	threadStats := &NfsdThreadStats{
		Threads: 16,
	}

	// Populate NfsdNetStats with sample data
	netStats := &NfsdNetStats{
		NetCount:           25000,
		UDPCount:           0,
		TCPCount:           25000,
		TCPConnectionCount: 128,
	}

	// Populate NfsdRPCStats with sample data
	rpcStats := &NfsdRPCStats{
		RPCCount:       30000,
		BadCount:       15,
		BadFmtCount:    5,
		BadAuthCount:   5,
		BadClientCount: 5,
	}

	// Populate NfsdV3ProcedureStats with a slice of sample CallStats
	v3ProcStats := &[]CallStats{
		{NFSVersion: 3, NFSCallName: "GETATTR", NFSCallCount: 4500},
		{NFSVersion: 3, NFSCallName: "SETATTR", NFSCallCount: 800},
		{NFSVersion: 3, NFSCallName: "LOOKUP", NFSCallCount: 3200},
		{NFSVersion: 3, NFSCallName: "ACCESS", NFSCallCount: 4000},
		{NFSVersion: 3, NFSCallName: "READ", NFSCallCount: 6000},
		{NFSVersion: 3, NFSCallName: "WRITE", NFSCallCount: 5500},
	}

	// Populate NfsdV4ProcedureStats with a sample CallStats for COMPOUND
	// In NFSv4, most operations are wrapped in a single COMPOUND procedure.
	v4ProcStats := &[]CallStats{
		{NFSVersion: 4, NFSCallName: "COMPOUND", NFSCallCount: 15000},
	}

	// Populate NfsdV4OperationStats with a slice of sample CallStats for v4 operations
	v4OpStats := &[]CallStats{
		{NFSVersion: 4, NFSCallName: "ACCESS", NFSCallCount: 2800},
		{NFSVersion: 4, NFSCallName: "GETATTR", NFSCallCount: 3500},
		{NFSVersion: 4, NFSCallName: "PUTFH", NFSCallCount: 3000},
		{NFSVersion: 4, NFSCallName: "READ", NFSCallCount: 2500},
		{NFSVersion: 4, NFSCallName: "WRITE", NFSCallCount: 2000},
		{NFSVersion: 4, NFSCallName: "COMMIT", NFSCallCount: 1200},
	}

	// Assemble the complete NfsdStats struct with all populated substructures
	stats := &NfsdStats{
		NfsdRepcacheStats:    repcacheStats,
		NfsdFhStats:          fhStats,
		NfsdIoStats:          ioStats,
		NfsdThreadStats:      threadStats,
		NfsdNetStats:         netStats,
		NfsdRPCStats:         rpcStats,
		NfsdV3ProcedureStats: v3ProcStats,
		NfsdV4ProcedureStats: v4ProcStats,
		NfsdV4OperationStats: v4OpStats,
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

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			noAttrs := pcommon.NewMap()
			assertMetric(t, md, "system.nfs.net.count", int64(scraper.nfsStats.NfsNetStats.NetCount), noAttrs)
			assertMetric(t, md, "system.nfs.net.udp.count", int64(scraper.nfsStats.NfsNetStats.UDPCount), noAttrs)
			assertMetric(t, md, "system.nfs.net.tcp.count", int64(scraper.nfsStats.NfsNetStats.TCPCount), noAttrs)
			assertMetric(t, md, "system.nfs.net.tcp.connections", int64(scraper.nfsStats.NfsNetStats.TCPConnectionCount), noAttrs)

			assertMetric(t, md, "system.nfs.rpc.count", int64(scraper.nfsStats.NfsRPCStats.RPCCount), noAttrs)
			assertMetric(t, md, "system.nfs.rpc.retransmits", int64(scraper.nfsStats.NfsRPCStats.RetransmitCount), noAttrs)
			assertMetric(t, md, "system.nfs.rpc.auth_refreshes", int64(scraper.nfsStats.NfsRPCStats.AuthRefreshCount), noAttrs)

			for _, s := range *scraper.nfsStats.NfsV3ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.procedure", s.NFSCallName)
				assertMetric(t, md, "system.nfs.procedure.count", int64(s.NFSCallCount), attrs)
			}

			for _, s := range *scraper.nfsStats.NfsV4ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.procedure", s.NFSCallName)
				assertMetric(t, md, "system.nfs.procedure.count", int64(s.NFSCallCount), attrs)
			}

			for _, s := range *scraper.nfsStats.NfsV4OperationStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.operation", s.NFSCallName)
				assertMetric(t, md, "system.nfs.operation.count", int64(s.NFSCallCount), attrs)
			}

			assertMetric(t, md, "system.nfsd.repcache.hits", int64(scraper.nfsdStats.NfsdRepcacheStats.Hits), noAttrs)
			assertMetric(t, md, "system.nfsd.repcache.misses", int64(scraper.nfsdStats.NfsdRepcacheStats.Misses), noAttrs)
			assertMetric(t, md, "system.nfsd.repcache.nocache", int64(scraper.nfsdStats.NfsdRepcacheStats.Nocache), noAttrs)

			assertMetric(t, md, "system.nfsd.fh.stale", int64(scraper.nfsdStats.NfsdFhStats.Stale), noAttrs)

			assertMetric(t, md, "system.nfsd.io.read", int64(scraper.nfsdStats.NfsdIoStats.Read), noAttrs)
			assertMetric(t, md, "system.nfsd.io.write", int64(scraper.nfsdStats.NfsdIoStats.Write), noAttrs)

			assertMetric(t, md, "system.nfsd.thread.count", int64(scraper.nfsdStats.NfsdThreadStats.Threads), noAttrs)

			assertMetric(t, md, "system.nfsd.net.count", int64(scraper.nfsdStats.NfsdNetStats.NetCount), noAttrs)
			assertMetric(t, md, "system.nfsd.net.udp.count", int64(scraper.nfsdStats.NfsdNetStats.UDPCount), noAttrs)
			assertMetric(t, md, "system.nfsd.net.tcp.count", int64(scraper.nfsdStats.NfsdNetStats.TCPCount), noAttrs)
			assertMetric(t, md, "system.nfsd.net.tcp.connections", int64(scraper.nfsdStats.NfsdNetStats.TCPConnectionCount), noAttrs)

			assertMetric(t, md, "system.nfsd.rpc.count", int64(scraper.nfsdStats.NfsdRPCStats.RPCCount), noAttrs)
			assertMetric(t, md, "system.nfsd.rpc.bad_calls", int64(scraper.nfsdStats.NfsdRPCStats.BadCount), noAttrs)
			assertMetric(t, md, "system.nfsd.rpc.bad_format", int64(scraper.nfsdStats.NfsdRPCStats.BadFmtCount), noAttrs)
			assertMetric(t, md, "system.nfsd.rpc.bad_auth", int64(scraper.nfsdStats.NfsdRPCStats.BadAuthCount), noAttrs)
			assertMetric(t, md, "system.nfsd.rpc.bad_clients", int64(scraper.nfsdStats.NfsdRPCStats.BadClientCount), noAttrs)

			for _, s := range *scraper.nfsdStats.NfsdV3ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.procedure", s.NFSCallName)
				assertMetric(t, md, "system.nfsd.procedure.count", int64(s.NFSCallCount), attrs)
			}

			for _, s := range *scraper.nfsdStats.NfsdV4ProcedureStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.procedure", s.NFSCallName)
				assertMetric(t, md, "system.nfsd.procedure.count", int64(s.NFSCallCount), attrs)
			}
			for _, s := range *scraper.nfsdStats.NfsdV4OperationStats {
				attrs := pcommon.NewMap()
				attrs.PutInt("nfs.version", s.NFSVersion)
				attrs.PutStr("nfs.operation", s.NFSCallName)
				assertMetric(t, md, "system.nfsd.operation.count", int64(s.NFSCallCount), attrs)
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
