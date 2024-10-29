// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal/metadata"
)

func TestTypeStr(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings()
	mReceiver, err := factory.CreateMetrics(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	tReceiver, err := factory.CreateTraces(context.Background(), set, cfg, nil)
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)
}
