// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/oraclecloud"
	rdpmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

var _ oraclecloud.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	out *oraclecloud.ComputeMetadata
	err error
}

func (m *mockMetadata) Metadata(_ context.Context) (*oraclecloud.ComputeMetadata, error) {
	return m.out, m.err
}

// Patches the IsRunningOnOracleCloudFunc to control probe results for the duration of a test.
// probeValue simulates whether the code "sees" Oracle Cloud (true = on platform, false = off platform).
// Automatically restores the original probe after the test.
func withOracleCloudProbe(t *testing.T, probeValue bool, testFunc func()) {
	origProbe := oraclecloud.IsRunningOnOracleCloudFunc
	oraclecloud.IsRunningOnOracleCloudFunc = func(context.Context) bool { return probeValue }
	t.Cleanup(func() { oraclecloud.IsRunningOnOracleCloudFunc = origProbe })
	testFunc()
}

// Validates successful detection and attribute population when simulating Oracle Cloud
// environment and valid metadata are present. Checks that all expected attributes and schemaURL are set.
func TestDetect(t *testing.T) {
	withOracleCloudProbe(t, true, func() {
		md := &mockMetadata{
			out: &oraclecloud.ComputeMetadata{
				HostID:             "ocid1.instance.oc1..aaaaaaa",
				HostDisplayName:    "my-instance",
				HostType:           "VM.Standard.E4.Flex",
				RegionID:           "us-ashburn-1",
				AvailabilityDomain: "AD-1",
				Metadata: oraclecloud.InstanceMetadata{
					OKEClusterDisplayName: "my-oke-cluster",
				},
			},
		}
		cfg := CreateDefaultConfig()

		det, err := NewDetector(processortest.NewNopSettings(rdpmetadata.Type), cfg)
		require.NoError(t, err)
		det.(*Detector).provider = md

		res, schemaURL, err := det.Detect(t.Context())
		require.NoError(t, err)
		assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")

		// Per Otel semantic conventions, these are the attribute keys for K8s clusters:
		// https://opentelemetry.io/docs/specs/semconv/resource/k8s/#cluster
		// We hardcode them here because no Go constant exists in semconv as of this writing.
		expected := map[string]any{
			"cloud.provider":          "oracle_cloud",
			"cloud.platform":          "oracle_cloud_oke",
			"cloud.region":            "us-ashburn-1",
			"cloud.availability_zone": "AD-1",
			"host.id":                 "ocid1.instance.oc1..aaaaaaa",
			"host.name":               "my-instance",
			"host.type":               "VM.Standard.E4.Flex",
			"k8s.cluster.name":        "my-oke-cluster",
		}
		assert.Equal(t, expected, res.Attributes().AsRaw())
	})
}

// Verifies that if the fast probe does not detect Oracle Cloud (simulated using mock probe),
// the detector returns an empty resource and no error.
func TestDetect_ProbeFails_ReturnsEmptyResourceNoError(t *testing.T) {
	withOracleCloudProbe(t, false, func() {
		cfg := CreateDefaultConfig()
		det, err := NewDetector(processortest.NewNopSettings(rdpmetadata.Type), cfg)
		require.NoError(t, err)

		res, schemaURL, err := det.Detect(t.Context())
		require.NoError(t, err)
		assert.Empty(t, res.Attributes().AsRaw())
		assert.Empty(t, schemaURL)
	})
}

// Verifies that if the probe is positive, but metadata fetch fails,
// the detector returns an error and no resource attributes.
func TestDetect_ProbeSucceeds_MetadataFails_ReturnsError(t *testing.T) {
	withOracleCloudProbe(t, true, func() {
		// Set up mock provider returning failure
		md := &mockMetadata{
			out: nil,
			err: assert.AnError,
		}
		cfg := CreateDefaultConfig()
		det, err := NewDetector(processortest.NewNopSettings(rdpmetadata.Type), cfg)
		require.NoError(t, err)
		det.(*Detector).provider = md

		res, schemaURL, err := det.Detect(t.Context())
		require.Error(t, err)
		assert.Empty(t, res.Attributes().AsRaw())
		assert.Empty(t, schemaURL)
	})
}

// Ensures disabling certain resource attributes results in them being omitted from the resource, when on Oracle Cloud.
func TestDetectDisabledResourceAttributes(t *testing.T) {
	withOracleCloudProbe(t, true, func() {
		md := &mockMetadata{
			out: &oraclecloud.ComputeMetadata{
				HostID:             "ocid1.instance.oc1..aaaaaaa",
				HostDisplayName:    "my-instance",
				HostType:           "VM.Standard.E4.Flex",
				RegionID:           "us-ashburn-1",
				AvailabilityDomain: "AD-1",
				Metadata: oraclecloud.InstanceMetadata{
					OKEClusterDisplayName: "my-oke-cluster",
				},
			},
		}
		cfg := CreateDefaultConfig()
		cfg.ResourceAttributes.K8sClusterName.Enabled = false

		det, err := NewDetector(processortest.NewNopSettings(rdpmetadata.Type), cfg)
		require.NoError(t, err)
		det.(*Detector).provider = md

		res, schemaURL, err := det.Detect(t.Context())
		require.NoError(t, err)
		assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")

		expected := map[string]any{
			"cloud.provider":          "oracle_cloud",
			"cloud.platform":          "oracle_cloud_oke",
			"cloud.region":            "us-ashburn-1",
			"cloud.availability_zone": "AD-1",
			"host.id":                 "ocid1.instance.oc1..aaaaaaa",
			"host.name":               "my-instance",
			"host.type":               "VM.Standard.E4.Flex",
			// K8S attributes omitted as they are disabled
		}
		assert.Equal(t, expected, res.Attributes().AsRaw())
	})
}
