// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !omit_detector_aws_eks

package eks

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	apiprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

func TestSetClusterARN(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		failOnMissing bool
		arn           string
		arnErr        error
		wantARN       string
		wantError     bool
		wantCalls     []string
	}{
		{
			name: "disabled",
		},
		{
			name:      "enabled",
			enabled:   true,
			arn:       "arn:aws:eks:us-west-2:123456789012:cluster/example",
			wantARN:   "arn:aws:eks:us-west-2:123456789012:cluster/example",
			wantCalls: []string{"example"},
		},
		{
			name:      "lookup failure is omitted",
			enabled:   true,
			arnErr:    errors.New("issuer mismatch"),
			wantCalls: []string{"example"},
		},
		{
			name:          "lookup failure is fatal when required",
			enabled:       true,
			failOnMissing: true,
			arnErr:        errors.New("issuer mismatch"),
			wantError:     true,
			wantCalls:     []string{"example"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := metadata.DefaultResourceAttributesConfig()
			attributes.AwsEksClusterArn.Enabled = tt.enabled
			provider := &mockAPIProvider{
				apiMeta:       apiprovider.InstanceMetadata{ClusterName: "example"},
				clusterARN:    tt.arn,
				clusterARNErr: tt.arnErr,
			}
			detector := &detector{
				logger:                zaptest.NewLogger(t),
				apiProvider:           provider,
				ra:                    attributes,
				rb:                    metadata.NewResourceBuilder(attributes),
				failOnMissingMetadata: tt.failOnMissing,
			}

			err := detector.setClusterARN(t.Context(), "us-west-2", "example")

			if tt.wantError {
				require.Error(t, err)
				assert.ErrorContains(t, err, "eks cluster ARN unavailable")
			} else {
				require.NoError(t, err)
			}
			resource := detector.rb.Emit()
			value, ok := resource.Attributes().Get("aws.eks.cluster.arn")
			if tt.wantARN == "" {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, tt.wantARN, value.Str())
			}
			assert.Equal(t, tt.wantCalls, provider.clusterARNCalls)
		})
	}
}

func TestDetectFromIMDSCollectsClusterARN(t *testing.T) {
	attributes := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
	}
	provider := &mockAPIProvider{
		apiMeta:    apiprovider.InstanceMetadata{ClusterName: "example"},
		clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/example",
	}
	detector := &detector{
		logger: zaptest.NewLogger(t),
		imdsProvider: &mockIMDSProvider{meta: imds.InstanceIdentityDocument{
			InstanceID: "i-123",
			Region:     "us-west-2",
		}},
		apiProvider: provider,
		ra:          attributes,
		rb:          metadata.NewResourceBuilder(attributes),
	}

	resource, _, err := detector.detectFromIMDS(t.Context())

	require.NoError(t, err)
	value, ok := resource.Attributes().Get("aws.eks.cluster.arn")
	require.True(t, ok)
	assert.Equal(t, provider.clusterARN, value.Str())
	assert.Equal(t, []string{"example"}, provider.clusterARNCalls)
	assert.Equal(t, []string{"us-west-2"}, provider.clusterARNRegions)
}

func TestDetectFromAPICollectsClusterARN(t *testing.T) {
	attributes := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
	}
	provider := &mockAPIProvider{
		nodeName:   "node",
		k8sMeta:    apiprovider.InstanceMetadata{InstanceID: "i-123", Region: "us-west-2"},
		apiMeta:    apiprovider.InstanceMetadata{InstanceID: "i-123", Region: "us-west-2", ClusterName: "example"},
		clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/example",
	}
	detector := &detector{
		logger:      zaptest.NewLogger(t),
		apiProvider: provider,
		ra:          attributes,
		rb:          metadata.NewResourceBuilder(attributes),
	}

	resource, _, err := detector.detectFromAPI(t.Context())

	require.NoError(t, err)
	value, ok := resource.Attributes().Get("aws.eks.cluster.arn")
	require.True(t, ok)
	assert.Equal(t, provider.clusterARN, value.Str())
	assert.Equal(t, []string{"example"}, provider.clusterARNCalls)
	assert.Equal(t, []string{"us-west-2"}, provider.clusterARNRegions)
}

func TestSetClusterARNSkipsMissingClusterName(t *testing.T) {
	attributes := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
	}
	provider := &mockAPIProvider{}
	detector := &detector{
		logger:                zaptest.NewLogger(t),
		apiProvider:           provider,
		ra:                    attributes,
		rb:                    metadata.NewResourceBuilder(attributes),
		failOnMissingMetadata: true,
	}

	err := detector.setClusterARN(t.Context(), "us-west-2", "")

	require.NoError(t, err)
	assert.Empty(t, provider.clusterARNCalls)
}

func TestDetectFromIMDSSkipsClusterLookupByDefault(t *testing.T) {
	provider := &mockAPIProvider{}
	detector := &detector{
		logger: zaptest.NewLogger(t),
		imdsProvider: &mockIMDSProvider{meta: imds.InstanceIdentityDocument{
			InstanceID: "i-123",
			Region:     "us-west-2",
		}},
		apiProvider: provider,
		rb:          metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{}),
	}

	_, _, err := detector.detectFromIMDS(t.Context())

	require.NoError(t, err)
	assert.Zero(t, provider.instanceMetadataCalls)
	assert.Empty(t, provider.clusterARNCalls)
}

func TestDetectFromAPISkipsClusterARNByDefault(t *testing.T) {
	provider := &mockAPIProvider{
		nodeName: "node",
		k8sMeta:  apiprovider.InstanceMetadata{InstanceID: "i-123", Region: "us-west-2"},
		apiMeta:  apiprovider.InstanceMetadata{InstanceID: "i-123", Region: "us-west-2", ClusterName: "example"},
	}
	detector := &detector{
		logger:      zaptest.NewLogger(t),
		apiProvider: provider,
		rb:          metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{}),
	}

	_, _, err := detector.detectFromAPI(t.Context())

	require.NoError(t, err)
	assert.Empty(t, provider.clusterARNCalls)
	assert.Empty(t, provider.clusterARNRegions)
}
