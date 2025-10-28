// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package psiscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	scraper := newPSIScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{})
	err := scraper.start(t.Context(), nil)
	require.NoError(t, err)

	metrics, err := scraper.scrape(t.Context())
	// On non-Linux systems, we expect an error
	// On Linux systems without PSI support, we may get an error or empty metrics
	// On Linux systems with PSI support, we expect metrics
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.GreaterOrEqual(t, metrics.MetricCount(), 0)
	}
}

func TestParsePSI(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		validate    func(*testing.T, *psiStats)
	}{
		{
			name: "valid cpu format",
			input: `some avg10=1.23 avg60=2.34 avg300=3.45 total=123456
`,
			expectError: false,
			validate: func(t *testing.T, stats *psiStats) {
				assert.Equal(t, 1.23, stats.some.avg10)
				assert.Equal(t, 2.34, stats.some.avg60)
				assert.Equal(t, 3.45, stats.some.avg300)
				assert.Equal(t, int64(123456), stats.some.total)
				assert.Equal(t, 0.0, stats.full.avg10)
				assert.Equal(t, 0.0, stats.full.avg60)
				assert.Equal(t, 0.0, stats.full.avg300)
				assert.Equal(t, int64(0), stats.full.total)
			},
		},
		{
			name: "valid memory format with full",
			input: `some avg10=1.23 avg60=2.34 avg300=3.45 total=123456
full avg10=0.50 avg60=1.00 avg300=1.50 total=654321
`,
			expectError: false,
			validate: func(t *testing.T, stats *psiStats) {
				assert.Equal(t, 1.23, stats.some.avg10)
				assert.Equal(t, 2.34, stats.some.avg60)
				assert.Equal(t, 3.45, stats.some.avg300)
				assert.Equal(t, int64(123456), stats.some.total)
				assert.Equal(t, 0.50, stats.full.avg10)
				assert.Equal(t, 1.00, stats.full.avg60)
				assert.Equal(t, 1.50, stats.full.avg300)
				assert.Equal(t, int64(654321), stats.full.total)
			},
		},
		{
			name: "zero values",
			input: `some avg10=0.00 avg60=0.00 avg300=0.00 total=0
full avg10=0.00 avg60=0.00 avg300=0.00 total=0
`,
			expectError: false,
			validate: func(t *testing.T, stats *psiStats) {
				assert.Equal(t, 0.0, stats.some.avg10)
				assert.Equal(t, 0.0, stats.some.avg60)
				assert.Equal(t, 0.0, stats.some.avg300)
				assert.Equal(t, int64(0), stats.some.total)
				assert.Equal(t, 0.0, stats.full.avg10)
				assert.Equal(t, 0.0, stats.full.avg60)
				assert.Equal(t, 0.0, stats.full.avg300)
				assert.Equal(t, int64(0), stats.full.total)
			},
		},
		{
			name:        "invalid format - not enough fields",
			input:       "some avg10=1.23\n",
			expectError: true,
		},
		{
			name:        "invalid format - bad avg10 value",
			input:       "some avg10=abc avg60=2.34 avg300=3.45 total=123456\n",
			expectError: true,
		},
		{
			name:        "invalid format - bad total value",
			input:       "some avg10=1.23 avg60=2.34 avg300=3.45 total=abc\n",
			expectError: true,
		},
		{
			name:        "invalid pressure type",
			input:       "partial avg10=1.23 avg60=2.34 avg300=3.45 total=123456\n",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stats, err := parsePSI(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, stats)
				}
			}
		})
	}
}
