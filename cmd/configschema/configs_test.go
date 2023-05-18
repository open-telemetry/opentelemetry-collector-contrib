// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package configschema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func TestGetAllConfigs(t *testing.T) {
	cfgs := GetAllCfgInfos(testComponents())
	require.NotNil(t, cfgs)
}

func TestCreateReceiverConfig(t *testing.T) {
	cfg, err := GetCfgInfo(testComponents(), "receiver", "otlp")
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestCreateProcesorConfig(t *testing.T) {
	cfg, err := GetCfgInfo(testComponents(), "processor", "filter")
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name          string
		componentType string
	}{
		{
			name:          "otlp",
			componentType: "receiver",
		},
		{
			name:          "filter",
			componentType: "processor",
		},
		{
			name:          "otlp",
			componentType: "exporter",
		},
		{
			name:          "zpages",
			componentType: "extension",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := GetCfgInfo(testComponents(), test.componentType, test.name)
			require.NoError(t, err)
			require.NotNil(t, cfg)
		})
	}
}

func testComponents() otelcol.Factories {
	cmps, err := components.Components()
	if err != nil {
		panic(err)
	}
	return cmps
}
