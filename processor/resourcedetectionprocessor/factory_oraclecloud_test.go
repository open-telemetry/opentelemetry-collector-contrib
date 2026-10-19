// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors && enable_resourcedetection_oraclecloud_detector && !omit_detector_oraclecloud

package resourcedetectionprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestOracleCloudOnlyBuildCreatesConfiguredProcessor(t *testing.T) {
	require.Contains(t, availableDetectorFactories(), internal.DetectorType("oraclecloud"))

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"oraclecloud"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, tp)
}

func TestOracleCloudOnlyBuildRejectsDisabledDetector(t *testing.T) {
	if _, ok := availableDetectorFactories()[internal.DetectorType("env")]; ok {
		t.Skip("env is compiled in this detector combination")
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"env"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, `detector "env" is not compiled into this binary`)
	require.ErrorContains(t, err, "oraclecloud")
	assert.Nil(t, tp)
}

func TestOracleCloudDetectorRegistry(t *testing.T) {
	for detectorType, factory := range availableDetectorFactories() {
		require.NotNil(t, factory, "factory should not be nil for %q", detectorType)
	}

	assert.Contains(t, availableDetectorFactories(), internal.DetectorType("oraclecloud"))
}
