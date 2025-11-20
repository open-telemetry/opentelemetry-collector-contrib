// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package macosunifiedloggingreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver/internal/metadata"
)

func TestDefaultConfigFailure(t *testing.T) {
	factory := newFactoryAdapter()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))

	receiver, err := factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		new(consumertest.LogsSink),
	)
	require.ErrorContains(t, err, "macosunifiedloggingreceiver receiver is only supported on macOS")
	require.Nil(t, receiver, "failed to create receiver")
}
