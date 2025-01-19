// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateTraces(t *testing.T) {
	// TODO review if test should succeed on Windows
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	t.Setenv(defaultRegionEnvName, mockRegion)

	factory := NewFactory()
	_, err := factory.CreateTraces(
		context.Background(),
		receivertest.NewNopSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.NoError(t, err, "trace receiver can be created")
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.Error(t, err, "a trace receiver factory should not create a metric receiver")
	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
}
