// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.18.0"
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
			expectedSchemaURL: conventions.SchemaURL,
		},
		{
			name:              "error getting k8s cluster version",
			detector:          newTestDetector(t, &providerResponse{}, nil, someErr, nil),
			expectedErr:       someErr,
			expectedResource:  pcommon.NewResource(),
			expectedSchemaURL: conventions.SchemaURL,
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
				attrs.PutStr(string(conventions.K8SClusterNameKey), "test-d-bm4rt")
				attrs.PutStr(string(conventions.CloudProviderKey), "aws")
				attrs.PutStr(string(conventions.CloudPlatformKey), "aws_openshift")
				attrs.PutStr(string(conventions.CloudRegionKey), "us-east-1")
				return res
			}(),
			expectedSchemaURL: conventions.SchemaURL,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resource, schemaURL, err := tc.detector.Detect(context.Background())
			if err != nil && errors.Is(err, tc.expectedErr) {
				return
			} else if err != nil && !errors.Is(err, tc.expectedErr) {
				t.Fatal(err)
			}

			assert.Equal(t, tc.expectedResource, resource)
			assert.Equal(t, tc.expectedSchemaURL, schemaURL)
		})
	}
}
