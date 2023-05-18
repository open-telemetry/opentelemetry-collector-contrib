// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestWithValidConfig(t *testing.T) {
	mockJarVersions()
	defer unmockJarVersions()

	f := NewFactory()
	assert.Equal(t, component.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "myendpoint:12345"
	cfg.(*Config).JARPath = "testdata/fake_jmx.jar"
	cfg.(*Config).TargetSystem = "jvm"

	params := receivertest.NewNopCreateSettings()
	r, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	receiver := r.(*jmxMetricReceiver)
	assert.Same(t, receiver.logger, params.Logger)
	assert.Same(t, receiver.config, cfg)
}
