// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package iisreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	mReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())

	assert.EqualError(t, err, "the windows perf counters receiver is only supported on Windows")
	assert.Nil(t, mReceiver)
}
