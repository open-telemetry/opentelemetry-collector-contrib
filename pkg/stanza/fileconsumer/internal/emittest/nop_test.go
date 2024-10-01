// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNop(t *testing.T) {
	require.NoError(t, Nop(context.Background(), nil, nil))
}
