// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestMergeMetrics(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"basic_merge",
	}

	for _, tc := range testCases {
		testName := tc

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join("testdata", testName)

			mdA, err := golden.ReadMetrics(filepath.Join(dir, "a.yaml"))
			require.NoError(t, err)

			mdB, err := golden.ReadMetrics(filepath.Join(dir, "b.yaml"))
			require.NoError(t, err)

			expectedOutput, err := golden.ReadMetrics(filepath.Join(dir, "output.yaml"))
			require.NoError(t, err)

			output := metrics.Merge(mdA, mdB)
			require.NoError(t, pmetrictest.CompareMetrics(expectedOutput, output))
		})
	}
}
