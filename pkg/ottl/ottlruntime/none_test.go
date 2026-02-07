// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoneValue(t *testing.T) {
	nv := NewNoneValue()
	require.NotNil(t, nv)
	assert.False(t, nv.IsNil())
}
