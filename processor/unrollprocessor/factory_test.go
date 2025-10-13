// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unrollprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, metadata.Type, factory.Type())

	expectedCfg := &Config{}

	cfg, ok := factory.CreateDefaultConfig().(*Config)
	require.True(t, ok)
	require.Equal(t, expectedCfg, cfg)
}
