// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestSinglePut(t *testing.T) {
	c, err := NewLRUDecisionCache(2)
	require.NoError(t, err)
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	c.Put(id, DecisionMetadata{
		PolicyName: "mock-policy",
	})
	v, ok := c.Get(id)
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy",
	}, v)
	assert.True(t, ok)
}

func TestExceedsSizeLimit(t *testing.T) {
	c, err := NewLRUDecisionCache(2)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	c.Put(id1, DecisionMetadata{
		PolicyName: "mock-policy1",
	})
	c.Put(id2, DecisionMetadata{
		PolicyName: "mock-policy2",
	})
	c.Put(id3, DecisionMetadata{
		PolicyName: "mock-policy3",
	})

	_, ok := c.Get(id1)
	assert.False(t, ok) // evicted
	v, ok := c.Get(id2)
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy2",
	}, v)
	assert.True(t, ok)
	v, ok = c.Get(id3)
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy3",
	}, v)
	assert.True(t, ok)
}

func TestLeastRecentlyUsedIsEvicted(t *testing.T) {
	c, err := NewLRUDecisionCache(2)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	c.Put(id1, DecisionMetadata{
		PolicyName: "mock-policy1",
	})
	c.Put(id2, DecisionMetadata{
		PolicyName: "mock-policy2",
	})
	v, ok := c.Get(id1) // use id1
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy1",
	}, v)
	assert.True(t, ok)
	c.Put(id3, DecisionMetadata{
		PolicyName: "mock-policy3",
	})

	v, ok = c.Get(id1)
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy1",
	}, v)
	assert.True(t, ok)
	_, ok = c.Get(id2)
	assert.False(t, ok) // evicted, not OK
	v, ok = c.Get(id3)
	assert.Equal(t, DecisionMetadata{
		PolicyName: "mock-policy3",
	}, v)
	assert.True(t, ok)
}

func traceIDFromHex(idStr string) (pcommon.TraceID, error) {
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	return id, err
}
