// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/metadata"
)

func TestWithValidConfig(t *testing.T) {
	mockJarVersions()
	defer unmockJarVersions()

	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "myendpoint:12345"
	cfg.(*Config).JARPath = "testdata/fake_jmx.jar"
	cfg.(*Config).TargetSystem = "jvm"

	params := receivertest.NewNopSettings()
	r, err := f.CreateMetrics(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	receiver := r.(*jmxMetricReceiver)
	assert.Same(t, receiver.logger, params.Logger)
	assert.Same(t, receiver.config, cfg)
}
