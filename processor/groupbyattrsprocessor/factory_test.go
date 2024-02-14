// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestDefaultConfiguration(t *testing.T) {
	c := createDefaultConfig().(*Config)
	assert.Empty(t, c.GroupByKeys)
}

func TestCreateTestProcessor(t *testing.T) {
	cfg := &Config{
		GroupByKeys: []string{"foo"},
	}

	tp, err := createTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.Equal(t, true, tp.Capabilities().MutatesData)

	lp, err := createLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
	assert.Equal(t, true, lp.Capabilities().MutatesData)

	mp, err := createMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	assert.Equal(t, true, mp.Capabilities().MutatesData)
}

func TestNoKeys(t *testing.T) {
	// This is allowed since can be used for compacting data
	gap, err := createGroupByAttrsProcessor(processortest.NewNopCreateSettings(), []string{})
	require.NoError(t, err)
	assert.NotNil(t, gap)
}

func TestDuplicateKeys(t *testing.T) {
	gbap, err := createGroupByAttrsProcessor(processortest.NewNopCreateSettings(), []string{"foo", "foo", ""})
	require.NoError(t, err)
	assert.NotNil(t, gbap)
	assert.EqualValues(t, []string{"foo"}, gbap.groupByKeys)
}
