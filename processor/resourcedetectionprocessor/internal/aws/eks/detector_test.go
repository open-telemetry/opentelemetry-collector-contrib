// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	apiprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

type mockIMDSProvider struct {
	meta     imds.InstanceIdentityDocument
	hostname string
	err      error
}

func (m *mockIMDSProvider) Get(_ context.Context) (imds.InstanceIdentityDocument, error) {
	return m.meta, m.err
}

func (m *mockIMDSProvider) Hostname(_ context.Context) (string, error) {
	return m.hostname, m.err
}

func (m *mockIMDSProvider) InstanceID(_ context.Context) (string, error) {
	return m.meta.InstanceID, m.err
}

type mockAPIProvider struct {
	clusterVersion    string
	clusterVersionErr error
	k8sMeta           apiprovider.InstanceMetadata
	k8sErr            error
	apiMeta           apiprovider.InstanceMetadata
	apiErr            error
	nodeName          string
}

func (m *mockAPIProvider) ClusterVersion() (string, error) {
	return m.clusterVersion, m.clusterVersionErr
}

func (m *mockAPIProvider) GetK8sInstanceMetadata(_ context.Context) (apiprovider.InstanceMetadata, error) {
	if m.nodeName == "" {
		return apiprovider.InstanceMetadata{}, errors.New("can't get K8s Instance Metadata; node name is empty")
	}
	return m.k8sMeta, m.k8sErr
}

func (m *mockAPIProvider) GetInstanceMetadata(ctx context.Context) (apiprovider.InstanceMetadata, error) {
	if m.apiMeta.Region == "" || m.apiMeta.InstanceID == "" {
		k8sMeta, err := m.GetK8sInstanceMetadata(ctx)
		if err != nil {
			return apiprovider.InstanceMetadata{}, err
		}
		m.apiMeta = apiprovider.InstanceMetadata{
			AvailabilityZone: k8sMeta.AvailabilityZone,
			Region:           k8sMeta.Region,
			InstanceID:       k8sMeta.InstanceID,
		}
	}
	return m.apiMeta, m.apiErr
}

func (m *mockAPIProvider) SetRegionInstanceID(region, instanceID string) {
	m.apiMeta.Region = region
	m.apiMeta.InstanceID = instanceID
}

func TestIsEKS(t *testing.T) {
	tests := []struct {
		name              string
		k8sHostEnvVar     string
		clusterVersion    string
		clusterVersionErr error
		isEKS             bool
		err               string
	}{
		{
			name:          "Not K8s Cluster",
			k8sHostEnvVar: "",
			isEKS:         false,
		},
		{
			name:              "ClusterVersion Error",
			k8sHostEnvVar:     "1",
			clusterVersionErr: errors.New("fail"),
			isEKS:             false,
			err:               "isEks() error retrieving cluster version:",
		},
		{
			name:           "ClusterVersion Not EKS",
			k8sHostEnvVar:  "1",
			clusterVersion: "v1.25.0",
			isEKS:          false,
		},
		{
			name:           "ClusterVersion EKS",
			k8sHostEnvVar:  "1",
			clusterVersion: "v1.32.3-eks-d0fe756",
			isEKS:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(kubernetesServiceHostEnvVar, tt.k8sHostEnvVar)

			d := &detector{
				apiProvider: &mockAPIProvider{
					clusterVersion:    tt.clusterVersion,
					clusterVersionErr: tt.clusterVersionErr,
				},
			}
			isEKS, err := d.isEKS()
			if tt.err != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.isEKS, isEKS)
		})
	}
}

func TestDetectFromIMDS(t *testing.T) {
	tests := []struct {
		name           string
		imdsMeta       imds.InstanceIdentityDocument
		apiMeta        apiprovider.InstanceMetadata
		hostname       string
		apiErr         error
		imdsErr        error
		expectedError  bool
		errMsg         string
		expectedOutput map[string]any
	}{
		{
			name: "Success",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			expectedError: false,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
		{
			name:          "IMDS Failure",
			imdsMeta:      imds.InstanceIdentityDocument{},
			apiMeta:       apiprovider.InstanceMetadata{},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			imdsErr:       errors.New("fail"),
			expectedError: true,
		},
		{
			name: "API Failure - Partial Result",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			apiErr:        errors.New("fail"),
			expectedError: true,
			expectedOutput: map[string]any{
				"host.id":   "i-123",
				"host.name": "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
		{
			name: "API Failure - Missing Region/InstanceID",
			imdsMeta: imds.InstanceIdentityDocument{
				AvailabilityZone: "us-west-2a",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			expectedError: true,
			errMsg:        "can't get K8s Instance Metadata; node name is empty",
			expectedOutput: map[string]any{
				"host.id":   "",
				"host.name": "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &detector{
				imdsProvider: &mockIMDSProvider{meta: tt.imdsMeta, hostname: tt.hostname, err: tt.imdsErr},
				apiProvider:  &mockAPIProvider{apiMeta: tt.apiMeta, apiErr: tt.apiErr},
				rb: metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{
					K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
					HostName:       metadata.ResourceAttributeConfig{Enabled: true},
					HostID:         metadata.ResourceAttributeConfig{Enabled: true},
				}),
				ra: metadata.ResourceAttributesConfig{K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true}},
			}
			res, schema, err := d.detectFromIMDS(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
			}
			if len(tt.expectedOutput) > 0 {
				assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			} else {
				assert.Equal(t, 0, res.Attributes().Len())
			}
			assert.Equal(t, conventions.SchemaURL, schema)
		})
	}
}

func TestDetectFromAPI(t *testing.T) {
	tests := []struct {
		name           string
		nodeName       string
		k8sMeta        apiprovider.InstanceMetadata
		apiMeta        apiprovider.InstanceMetadata
		k8sErr         error
		apiErr         error
		expectedError  bool
		expectedOutput map[string]any
	}{
		{
			name:     "Success",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				ClusterName:      "cluster",
				Hostname:         "i-123",
			},
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
		{
			name:     "K8s Metadata Error",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				ClusterName:      "cluster",
			},
			k8sErr:        errors.New("fail"),
			expectedError: true,
		},
		{
			name:     "API Metadata Error - Partial Result K8s Metadata",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta:       apiprovider.InstanceMetadata{},
			apiErr:        errors.New("fail"),
			expectedError: true,
			expectedOutput: map[string]any{
				"host.id": "i-123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &detector{
				apiProvider: &mockAPIProvider{
					k8sMeta:  tt.k8sMeta,
					apiMeta:  tt.apiMeta,
					k8sErr:   tt.k8sErr,
					apiErr:   tt.apiErr,
					nodeName: tt.nodeName,
				},
				rb: metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{
					K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
					HostName:       metadata.ResourceAttributeConfig{Enabled: true},
					HostID:         metadata.ResourceAttributeConfig{Enabled: true},
				}),
			}
			res, schema, err := d.detectFromAPI(context.Background())
			assert.Equal(t, conventions.SchemaURL, schema)
			if tt.expectedError {
				assert.Error(t, err)
			}
			if len(tt.expectedOutput) > 0 {
				assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			} else {
				assert.Equal(t, 0, res.Attributes().Len())
			}
		})
	}
}

type mockDetectorUtils struct {
	cfg            Config
	logger         *zap.Logger
	imdsAccessible bool
}

func (m *mockDetectorUtils) isIMDSAccessible(_ context.Context) bool {
	return m.imdsAccessible
}

func (m *mockDetectorUtils) getAWSConfig(_ context.Context, _ bool) (aws.Config, error) {
	return aws.Config{}, nil
}

func TestDetect(t *testing.T) {
	// Minimal config for detector
	cfg := Config{
		ResourceAttributes: metadata.ResourceAttributesConfig{
			K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
			HostName:       metadata.ResourceAttributeConfig{Enabled: true},
			HostID:         metadata.ResourceAttributeConfig{Enabled: true},
		},
	}
	t.Setenv(kubernetesServiceHostEnvVar, "1")

	set := processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	}

	det := &detector{
		cfg:    cfg,
		logger: set.Logger,
		apiProvider: &mockAPIProvider{
			clusterVersion: "v1.32.3-eks-d0fe756",
			nodeName:       "i-123",
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName:      "cluster",
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				Hostname:         "i-123",
			},
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
		},
		imdsProvider: &mockIMDSProvider{
			meta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			hostname: "i-123",
		},
		ra: cfg.ResourceAttributes,
		rb: metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}

	tests := []struct {
		name           string
		imdsAccessible bool
		expectedOutput map[string]any
	}{
		{
			name:           "Detect from API",
			imdsAccessible: false,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
		{
			name:           "Detect from IMDS",
			imdsAccessible: true,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			det.utils = &mockDetectorUtils{
				cfg:            cfg,
				logger:         set.Logger,
				imdsAccessible: tt.imdsAccessible,
			}
			res, schema, err := det.Detect(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, conventions.SchemaURL, schema)
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
		})
	}
}
