// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), component.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	conn, err := factory.CreateTracesProcessor(
		context.Background(),
		processortest.NewNopCreateSettings(),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
