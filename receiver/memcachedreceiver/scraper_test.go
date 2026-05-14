// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memcachedreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	scraper := newMemcachedScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	scraper.newClient = func(string, time.Duration) (client, error) {
		return &fakeClient{}, nil
	}

	actualMetrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "metrics.assert.yaml")
	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, actualMetrics))

	require.NoError(t, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
}
