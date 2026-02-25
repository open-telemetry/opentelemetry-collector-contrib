// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNopCache(t *testing.T) {
	c := NewNopDecisionCache()
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	c.Put(id, DecisionMetadata{
		PolicyName: "mock-policy",
	})
	_, ok := c.Get(id)
	assert.False(t, ok)
}
