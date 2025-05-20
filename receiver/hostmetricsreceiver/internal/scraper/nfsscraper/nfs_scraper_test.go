// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"context"
	"runtime"
	"testing"

	//	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	if !supportedOS {
                t.Skip()
        }

	ctx := context.Background()

	type testCase struct {
		name		string
		NfsScraperEnabled	bool
		NfsdScraperEnabled	bool
	}

	testCases := []testCase{
		{
			name: "NFS client metrics",
			NfsScraperEnabled: true,
			NfsdScraperEnabled: false,
		},
		{
			name: "NFS server metrics",
			NfsScraperEnabled: true,
			NfsdScraperEnabled: false,
		},
		{
			name: "All metrics",
			NfsScraperEnabled: true,
			NfsdScraperEnabled: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper, err := newNfsScraper(scrapertest.NewNopSettings(metadata.Type), &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			})
			require.NoError(t, err, "Failed to create NFS scraper: %v", err)

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			require.NoError(t, err)
		})
	}
}
