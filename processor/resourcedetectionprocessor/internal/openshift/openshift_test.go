// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap/zaptest"

	ocp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

type providerResponse struct {
	ocp.InfrastructureAPIResponse

	OpenShiftClusterVersion string
	K8SClusterVersion       string
}

type mockProvider struct {
	res      *providerResponse
	ocpCVErr error
	k8sCVErr error
	infraErr error
}

func (m *mockProvider) OpenShiftClusterVersion(context.Context) (string, error) {
	if m.ocpCVErr != nil {
		return "", m.ocpCVErr
	}
	return m.res.OpenShiftClusterVersion, nil
}

func (m *mockProvider) K8SClusterVersion(context.Context) (string, error) {
	if m.k8sCVErr != nil {
		return "", m.k8sCVErr
	}
	return m.res.K8SClusterVersion, nil
}

func (m *mockProvider) Infrastructure(context.Context) (*ocp.InfrastructureAPIResponse, error) {
	if m.infraErr != nil {
		return nil, m.infraErr
	}
	return &m.res.InfrastructureAPIResponse, nil
}

func newTestDetector(t *testing.T, res *providerResponse, ocpCVErr, k8sCVErr, infraErr error) internal.Detector {
	return &detector{
		logger: zaptest.NewLogger(t),
		provider: &mockProvider{
			res:      res,
			ocpCVErr: ocpCVErr,
			k8sCVErr: k8sCVErr,
			infraErr: infraErr,
		},
		rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}
}

func TestDetect(t *testing.T) {
	someErr := errors.New("test")
	tt := []struct {
		name              string
		detector          internal.Detector
		expectedResource  pcommon.Resource
		expectedSchemaURL string
		expectedErr       error
	}{
		{
			name:              "error getting openshift cluster version",
			detector:          newTestDetector(t, &providerResponse{}, someErr, nil, nil),
			expectedErr:       someErr,
			expectedResource:  pcommon.NewResource(),
			expectedSchemaURL: "https://opentelemetry.io/schemas/",
		},
		{
			name:              "error getting k8s cluster version",
			detector:          newTestDetector(t, &providerResponse{}, nil, someErr, nil),
			expectedErr:       someErr,
			expectedResource:  pcommon.NewResource(),
			expectedSchemaURL: "https://opentelemetry.io/schemas/",
		},
		{
			name:             "error getting infrastructure details",
			detector:         newTestDetector(t, &providerResponse{}, nil, nil, someErr),
			expectedErr:      someErr,
			expectedResource: pcommon.NewResource(),
		},
		{
			name: "detect all details",
			detector: newTestDetector(t, &providerResponse{
				InfrastructureAPIResponse: ocp.InfrastructureAPIResponse{
					Status: ocp.InfrastructureStatus{
						InfrastructureName:     "test-d-bm4rt",
						ControlPlaneTopology:   "HighlyAvailable",
						InfrastructureTopology: "HighlyAvailable",
						PlatformStatus: ocp.InfrastructurePlatformStatus{
							Type: "AWS",
							Aws: ocp.InfrastructureStatusAWS{
								Region: "us-east-1",
							},
						},
					},
				},
				OpenShiftClusterVersion: "4.1.2",
				K8SClusterVersion:       "1.23.4",
			}, nil, nil, nil),
			expectedErr: nil,
			expectedResource: func() pcommon.Resource {
				res := pcommon.NewResource()
				attrs := res.Attributes()
				attrs.PutStr("k8s.cluster.name", "test-d-bm4rt")
				attrs.PutStr("cloud.provider", "aws")
				attrs.PutStr("cloud.platform", "aws_openshift")
				attrs.PutStr("cloud.region", "us-east-1")
				return res
			}(),
			expectedSchemaURL: "https://opentelemetry.io/schemas/",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resource, schemaURL, err := tc.detector.Detect(t.Context())
			if err != nil && errors.Is(err, tc.expectedErr) {
				return
			} else if err != nil && !errors.Is(err, tc.expectedErr) {
				t.Fatal(err)
			}

			assert.Equal(t, tc.expectedResource, resource)
			assert.Contains(t, schemaURL, tc.expectedSchemaURL)
		})
	}
}

func TestDetectPlatforms(t *testing.T) {
	tt := []struct {
		name           string
		infraName      string
		platformStatus ocp.InfrastructurePlatformStatus
		expected       map[string]any
	}{
		{
			name:      "azure",
			infraName: "test-cluster",
			platformStatus: ocp.InfrastructurePlatformStatus{
				Type:  "Azure",
				Azure: ocp.InfrastructureStatusAzure{CloudName: "AzurePublicCloud"},
			},
			expected: map[string]any{
				"k8s.cluster.name": "test-cluster",
				"cloud.provider":   "azure",
				"cloud.platform":   "azure.openshift",
				"cloud.region":     "azurepubliccloud",
			},
		},
		{
			name:      "gcp",
			infraName: "test-cluster",
			platformStatus: ocp.InfrastructurePlatformStatus{
				Type: "GCP",
				GCP:  ocp.InfrastructureStatusGCP{Region: "US-Central1"},
			},
			expected: map[string]any{
				"k8s.cluster.name": "test-cluster",
				"cloud.provider":   "gcp",
				"cloud.platform":   "gcp_openshift",
				"cloud.region":     "us-central1",
			},
		},
		{
			name:      "ibmcloud",
			infraName: "test-cluster",
			platformStatus: ocp.InfrastructurePlatformStatus{
				Type:     "IBMCloud",
				IBMCloud: ocp.InfrastructureStatusIBMCloud{Location: "EU-DE"},
			},
			expected: map[string]any{
				"k8s.cluster.name": "test-cluster",
				"cloud.provider":   "ibm_cloud",
				"cloud.platform":   "ibm_cloud_openshift",
				"cloud.region":     "eu-de",
			},
		},
		{
			name:      "openstack sets only region",
			infraName: "test-cluster",
			platformStatus: ocp.InfrastructurePlatformStatus{
				Type:      "OpenStack",
				OpenStack: ocp.InfrastructureStatusOpenStack{CloudName: "MyCloud"},
			},
			expected: map[string]any{
				"k8s.cluster.name": "test-cluster",
				"cloud.region":     "mycloud",
			},
		},
		{
			name:      "unknown platform sets only cluster name",
			infraName: "test-cluster",
			platformStatus: ocp.InfrastructurePlatformStatus{
				Type: "BareMetal",
			},
			expected: map[string]any{
				"k8s.cluster.name": "test-cluster",
			},
		},
		{
			name:     "empty infrastructure name and platform",
			expected: map[string]any{},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDetector(t, &providerResponse{
				InfrastructureAPIResponse: ocp.InfrastructureAPIResponse{
					Status: ocp.InfrastructureStatus{
						InfrastructureName: tc.infraName,
						PlatformStatus:     tc.platformStatus,
					},
				},
			}, nil, nil, nil)
			res, schemaURL, err := d.Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expected, res.Attributes().AsRaw())
			assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
		})
	}
}

func TestDetectFailOnMissingMetadata(t *testing.T) {
	infraErr := errors.New("connection refused")
	d := &detector{
		logger:                zaptest.NewLogger(t),
		provider:              &mockProvider{res: &providerResponse{}, infraErr: infraErr},
		rb:                    metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
		failOnMissingMetadata: true,
	}
	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, infraErr)
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

func TestNewDetector(t *testing.T) {
	tt := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "explicit address and token",
			cfg: Config{
				Address: "https://api.example.com:6443",
				Token:   "token",
				TLSs:    configtls.ClientConfig{Insecure: true},
			},
		},
		{
			name: "invalid CA file",
			cfg: Config{
				Address: "https://api.example.com:6443",
				Token:   "token",
				TLSs:    configtls.ClientConfig{Config: configtls.Config{CAFile: "/non/existent/ca.crt"}},
			},
			wantErr: true,
		},
		{
			name:    "missing token and no in-cluster token file",
			cfg:     Config{Address: "https://api.example.com:6443"},
			wantErr: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.ResourceAttributes = metadata.DefaultResourceAttributesConfig()
			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tc.cfg, false)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, d)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, d)
		})
	}
}
