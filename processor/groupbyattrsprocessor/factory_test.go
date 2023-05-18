// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
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
	gap := createGroupByAttrsProcessor(zap.NewNop(), []string{})
	assert.NotNil(t, gap)
}

func TestDuplicateKeys(t *testing.T) {
	gbap := createGroupByAttrsProcessor(zap.NewNop(), []string{"foo", "foo", ""})
	assert.NotNil(t, gbap)
	assert.EqualValues(t, []string{"foo"}, gbap.groupByKeys)
}
