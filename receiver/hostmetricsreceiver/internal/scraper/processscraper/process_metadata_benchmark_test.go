// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/scraper"
)

func BenchmarkGetProcessMetadata(b *testing.B) {
	ctx := context.Background()
	config := &Config{}

	scraper, err := newProcessScraper(scraper.Settings{}, config)
	if err != nil {
		b.Fatalf("Failed to create process scraper: %v", err)
	}

	benchmarks := []struct {
		name             string
		useLegacy        bool
		parentPidEnabled bool
	}{
		{
			name: "New-ExcludeParentPid",
		},
		{
			name:      "Old-ExcludeParentPid",
			useLegacy: true,
		},
		{
			name:             "New-IncludeParentPid",
			parentPidEnabled: true,
		},
		{
			name:             "Old-IncludeParentPid",
			parentPidEnabled: true,
			useLegacy:        true,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Set feature gate value
			previousValue := useNewGetProcessHandles.IsEnabled()
			require.NoError(b, featuregate.GlobalRegistry().Set(useNewGetProcessHandles.ID(), !bm.useLegacy))
			defer func() {
				require.NoError(b, featuregate.GlobalRegistry().Set(useNewGetProcessHandles.ID(), previousValue))
			}()
			scraper.config.MetricsBuilderConfig.ResourceAttributes.ProcessParentPid.Enabled = bm.parentPidEnabled

			for i := 0; i < b.N; i++ {
				// Typically there are errors, but we are not interested in them for this benchmark
				_, _ = scraper.getProcessMetadata(ctx)
			}
		})
	}
}
