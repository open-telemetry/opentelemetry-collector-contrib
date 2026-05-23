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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
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

func (m *mockIMDSProvider) Tags(_ context.Context) ([]string, error) {
	return nil, m.err
}

func (m *mockIMDSProvider) Tag(_ context.Context, _ string) (string, error) {
	return "", m.err
}

type mockAPIProvider struct {
	clusterVersion    string
	clusterVersionErr error
	oidcIssuer        string
	oidcIssuerErr     error
	k8sMeta           apiprovider.InstanceMetadata
	k8sErr            error
	apiMeta           apiprovider.InstanceMetadata
	apiErr            error
	nodeName          string
}

func (m *mockAPIProvider) ClusterVersion() (string, error) {
	return m.clusterVersion, m.clusterVersionErr
}

func (m *mockAPIProvider) OIDCIssuer(_ context.Context) (string, error) {
	return m.oidcIssuer, m.oidcIssuerErr
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
		irsaTokenFile     string
		podIdentityFile   string
		oidcIssuer        string
		oidcIssuerErr     error
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
			name:          "IRSA Token Path",
			k8sHostEnvVar: "1",
			irsaTokenFile: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
			isEKS:         true,
		},
		{
			name:            "Pod Identity Token Path",
			k8sHostEnvVar:   "1",
			podIdentityFile: "/var/run/secrets/eks-pod-identity/serviceaccount/token",
			isEKS:           true,
		},
		{
			name:          "OIDC Issuer EKS",
			k8sHostEnvVar: "1",
			oidcIssuer:    "https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B716D3041E",
			isEKS:         true,
		},
		{
			name:           "OIDC Issuer Error Falls Through",
			k8sHostEnvVar:  "1",
			oidcIssuerErr:  errors.New("fail"),
			clusterVersion: "v1.32.3-eks-d0fe756",
			isEKS:          true,
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
			t.Setenv(awsWebIdentityTokenFileEnvVar, tt.irsaTokenFile)
			t.Setenv(awsContainerAuthTokenFileEnvVar, tt.podIdentityFile)

			d := &detector{
				apiProvider: &mockAPIProvider{
					clusterVersion:    tt.clusterVersion,
					clusterVersionErr: tt.clusterVersionErr,
					oidcIssuer:        tt.oidcIssuer,
					oidcIssuerErr:     tt.oidcIssuerErr,
				},
			}
			isEKS, err := d.isEKS(t.Context())
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
			res, schema, err := d.detectFromIMDS(t.Context())
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
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
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
			res, schema, err := d.detectFromAPI(t.Context())
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
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

func (*mockDetectorUtils) getAWSConfig(context.Context, bool) (aws.Config, error) {
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
			res, schema, err := det.Detect(t.Context())
			assert.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
		})
	}
}

// TestNewDetector_NonK8sHost is a fork-specific regression test for the
// PR #227 + PR #230 carry pair (now applied to the v0.150.x architecture
// where apiprovider.NewProvider replaces the old newK8sDetectorUtils as the
// non-K8s failure point).
//
// CWA's Application Signals translator unconditionally emits
// `detectors: [eks, env, ec2]` on every non-ECS platform. On non-EKS hosts
// (plain EC2, on-prem, self-managed K8s on EC2), rest.InClusterConfig() inside
// apiprovider.NewProvider() fails. NewDetector must NOT propagate that error
// and Detect() must return an empty resource without an error so the env+ec2
// detectors in the preset can run.
//
// Regression hazard: in the v0.143.0 bump (commit 71879b9, Feb 2026, out of
// scope) this carry was lost and re-applied in 3d13bfb after a Windows EC2
// integration test caught the resulting non-EKS crash-loop. Keep this test.
func TestNewDetector_NonK8sHost(t *testing.T) {
	// Ensure no K8s context in env so apiprovider.NewProvider's
	// rest.InClusterConfig() will fail.
	t.Setenv(kubernetesServiceHostEnvVar, "")
	t.Setenv(awsWebIdentityTokenFileEnvVar, "")
	t.Setenv(awsContainerAuthTokenFileEnvVar, "")

	dcfg := CreateDefaultConfig()
	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg)
	require.NoError(t, err, "NewDetector must tolerate non-K8s hosts (PR #227 + PR #230 carry)")
	require.NotNil(t, det)

	// Detect must short-circuit to an empty resource with no error when
	// apiProvider is nil.
	res, schemaURL, err := det.Detect(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, pcommon.NewResource().Attributes().AsRaw(), res.Attributes().AsRaw())
	assert.Empty(t, schemaURL)
}
