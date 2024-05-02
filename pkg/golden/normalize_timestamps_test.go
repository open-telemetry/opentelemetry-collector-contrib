// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTimestamps(t *testing.T) {
	dir := filepath.Join("testdata", "timestamp-norm")
	before, err := ReadMetrics(filepath.Join(dir, "before_normalize.yaml"))
	require.NoError(t, err)
	after, err := ReadMetrics(filepath.Join(dir, "after_normalize.yaml"))
	require.NoError(t, err)
	normalizeTimestamps(before)

	require.Equal(t, before, after)
}
