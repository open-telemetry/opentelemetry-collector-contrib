// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors && enable_detector_aws_ec2 && !omit_detector_aws_ec2

package resourcedetectionprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestEC2TrimmedBuildCreatesConfiguredProcessor(t *testing.T) {
	require.Contains(t, availableDetectorFactories(), internal.DetectorType("ec2"))
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Preserve the EC2-only configuration reported in issue #47218, including
	// the legacy per-detector fail_on_missing_metadata setting.
	cm := confmap.NewFromStringMap(map[string]any{
		"detectors": []any{"ec2"},
		"ec2":       map[string]any{"fail_on_missing_metadata": true},
	})
	require.NoError(t, cm.Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))
	assert.True(t, cfg.DetectorConfig.EC2Config.FailOnMissingMetadata) //nolint:staticcheck // Verify compatibility with the deprecated per-detector setting.

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
}
