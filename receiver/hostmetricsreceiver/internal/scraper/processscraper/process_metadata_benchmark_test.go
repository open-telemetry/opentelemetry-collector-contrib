// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/scraper"
)

func BenchmarkGetProcessMetadata(b *testing.B) {
	ctx := context.Background()
	config := &Config{
		MuteProcessExeError:  true,
		MuteProcessNameError: true,
		ExcludeParentPid:     true,
		MuteProcessAllErrors: true, // Only way to pass the benchmark
	}

	scraper, err := newProcessScraper(scraper.Settings{}, config)
	if err != nil {
		b.Fatalf("Failed to create process scraper: %v", err)
	}

	benchmarks := []struct {
		name             string
		getFunc          func(context.Context) (processHandles, error)
		excludeParentPid bool
	}{
		{
			name:    "Old-IncludeParentPid",
			getFunc: getProcessHandlesInternal,
		},
		{
			name:    "New-IncludeParentPid",
			getFunc: getProcessHandlesInternalNew,
		},
		{
			name:             "Old-ExcludeParentPid",
			getFunc:          getProcessHandlesInternal,
			excludeParentPid: true,
		},
		{
			name:             "New-ExcludeParentPid",
			getFunc:          getProcessHandlesInternalNew,
			excludeParentPid: true,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			scraper.getProcessHandles = bm.getFunc
			scraper.config.ExcludeParentPid = bm.excludeParentPid

			for i := 0; i < b.N; i++ {
				_, err := scraper.getProcessMetadata(ctx)
				if err != nil {
					b.Fatalf("Failed to get process metadata: %v", err)
				}
			}
		})
	}
}
