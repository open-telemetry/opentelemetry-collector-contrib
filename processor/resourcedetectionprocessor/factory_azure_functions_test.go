// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors && enable_detector_azure_functions && !omit_detector_azure_functions

package resourcedetectionprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestAzureFunctionsTrimmedBuildCreatesConfiguredProcessor(t *testing.T) {
	require.Contains(t, internal.NewProviderFactory(detectorFactories()).CompiledDetectorTypes(), "azurefunctions")
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Detectors = []string{"azurefunctions"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
}
