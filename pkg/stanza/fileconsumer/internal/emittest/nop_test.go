// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNop(t *testing.T) {
	require.NoError(t, Nop(t.Context(), [][]byte{}, map[string]any{}, int64(0), []int64{}))
}
