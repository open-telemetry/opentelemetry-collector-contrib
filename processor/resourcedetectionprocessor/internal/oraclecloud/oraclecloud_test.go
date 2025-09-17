// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.30.0"

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

func TestDetect(t *testing.T) {
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
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	// Per Otel semantic conventions, these are the attribute keys for K8s clusters:
	// https://opentelemetry.io/docs/specs/semconv/resource/k8s/#cluster
	// We hardcode them here because no Go constant exists in semconv as of this writing.
	expected := map[string]any{
		string(conventions.CloudProviderKey):         conventions.CloudProviderOracleCloud.Value.AsString(),
		string(conventions.CloudPlatformKey):         conventions.CloudPlatformOracleCloudOke.Value.AsString(),
		string(conventions.CloudRegionKey):           "us-ashburn-1",
		string(conventions.CloudAvailabilityZoneKey): "AD-1",
		string(conventions.HostIDKey):                "ocid1.instance.oc1..aaaaaaa",
		string(conventions.HostNameKey):              "my-instance",
		string(conventions.HostTypeKey):              "VM.Standard.E4.Flex",
		"k8s.cluster.name":                           "my-oke-cluster",
	}
	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
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
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	expected := map[string]any{
		string(conventions.CloudProviderKey):         conventions.CloudProviderOracleCloud.Value.AsString(),
		string(conventions.CloudPlatformKey):         conventions.CloudPlatformOracleCloudOke.Value.AsString(),
		string(conventions.CloudRegionKey):           "us-ashburn-1",
		string(conventions.CloudAvailabilityZoneKey): "AD-1",
		string(conventions.HostIDKey):                "ocid1.instance.oc1..aaaaaaa",
		string(conventions.HostNameKey):              "my-instance",
		string(conventions.HostTypeKey):              "VM.Standard.E4.Flex",
		// K8S attributes omitted as they are disabled
	}
	assert.Equal(t, expected, res.Attributes().AsRaw())
}
