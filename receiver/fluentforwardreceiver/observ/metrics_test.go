// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observ

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViews(t *testing.T) {
	require.Equal(t, len(MetricViews()), 5)
}
