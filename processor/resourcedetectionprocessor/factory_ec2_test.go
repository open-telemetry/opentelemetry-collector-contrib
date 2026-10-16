// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors && enable_resourcedetection_aws_ec2_detector && !omit_detector_aws_ec2

package resourcedetectionprocessor

import (
	"os/exec"
	"strings"
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

func TestEC2TrimmedBuildExcludesKubernetesDependencies(t *testing.T) {
	// Inspect production imports, not the test binary: existing config tests
	// import other detector implementations even in a targeted trimmed test run.
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps",
		"-tags=remove_all_resourcedetection_detectors,enable_resourcedetection_aws_ec2_detector", ".")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	packages := strings.Fields(string(output))
	const rdpInternal = "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/"
	require.Contains(t, packages, rdpInternal+"aws/ec2")
	for _, pkg := range packages {
		for _, prefix := range []string{"k8s.io/", "sigs.k8s.io/", "github.com/openshift/"} {
			assert.False(t, strings.HasPrefix(pkg, prefix), "EC2-only build imports %s", pkg)
		}
	}
}
