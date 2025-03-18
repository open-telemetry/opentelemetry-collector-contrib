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
	config := &Config{
		MuteProcessExeError:  true,
		MuteProcessNameError: true,
		MuteProcessAllErrors: true, // Only way to pass the benchmark
	}

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
			previousValue := useLegacyGetProcessHandles.IsEnabled()
			require.NoError(b, featuregate.GlobalRegistry().Set(useLegacyGetProcessHandles.ID(), bm.useLegacy))
			defer func() {
				require.NoError(b, featuregate.GlobalRegistry().Set(useLegacyGetProcessHandles.ID(), previousValue))
			}()
			scraper.config.MetricsBuilderConfig.ResourceAttributes.ProcessParentPid.Enabled = bm.parentPidEnabled

			for i := 0; i < b.N; i++ {
				_, err := scraper.getProcessMetadata(ctx)
				if err != nil {
					b.Fatalf("Failed to get process metadata: %v", err)
				}
			}
		})
	}
}
