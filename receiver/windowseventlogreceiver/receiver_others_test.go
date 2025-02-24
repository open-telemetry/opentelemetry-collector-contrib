// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package windowseventlogreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

func TestDefaultConfigFailure(t *testing.T) {
	factory := newFactoryAdapter()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))

	receiver, err := factory.CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		new(consumertest.LogsSink),
	)
	require.Nil(t, receiver, "failed to create default config")
	require.ErrorContains(t, err, "windows eventlog receiver is only supported on Windows")
}
