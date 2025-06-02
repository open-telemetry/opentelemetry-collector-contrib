// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"context"
	"testing"

	//	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
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

	ctx := context.Background()

	type testCase struct {
		name               string
	}

	testCases := []testCase{
		{
    			name:               "All metrics",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper :=&nfsScraper{
				settings:  scrapertest.NewNopSettings(metadata.Type),
				config:    &Config{
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				},
				nfsStats:  mockGetNfsStats,
				nfsdStats: mockGetNfsdStats,
			}
			
			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			
		})
	}

	nothing(ctx)
}
