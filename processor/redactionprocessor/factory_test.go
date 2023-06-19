// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestDefaultConfiguration(t *testing.T) {
	c := createDefaultConfig().(*Config)
	assert.Empty(t, c.AllowedKeys)
	assert.Empty(t, c.BlockedValues)
}

func TestCreateTestProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.Equal(t, true, tp.Capabilities().MutatesData)
}
