// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

const (
	clusterName    = "my-cluster"
	cloudAccountID = "cloud1234"
)

type MockDetectorUtils struct {
	mock.Mock
}

func (detectorUtils *MockDetectorUtils) getConfigMap(_ context.Context, namespace string, name string) (map[string]string, error) {
	args := detectorUtils.Called(namespace, name)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (detectorUtils *MockDetectorUtils) getClusterName(_ context.Context, _ *zap.Logger) string {
	var reservations []*ec2.Reservation
	return detectorUtils.getClusterNameTagFromReservations(reservations)
}

func (detectorUtils *MockDetectorUtils) getClusterNameTagFromReservations(_ []*ec2.Reservation) string {
	return clusterName
}

func (detectorUtils *MockDetectorUtils) getCloudAccountID(_ context.Context, _ *zap.Logger) string {
	return cloudAccountID
}

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	detector, err := NewDetector(processortest.NewNopSettings(), dcfg)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
	detectorUtils := new(MockDetectorUtils)
	ctx := context.Background()

	t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
	detectorUtils.On("getConfigMap", authConfigmapNS, authConfigmapName).Return(map[string]string{conventions.AttributeK8SClusterName: clusterName}, nil)
	// Call EKS Resource detector to detect resources
	eksResourceDetector := &detector{utils: detectorUtils, err: nil, ra: metadata.DefaultResourceAttributesConfig(), rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	res, _, err := eksResourceDetector.Detect(ctx)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider": "aws",
		"cloud.platform": "aws_eks",
	}, res.Attributes().AsRaw(), "Resource object returned is incorrect")
}

// Tests EKS resource detector not running in EKS environment by verifying resource is not running on k8s
func TestNotEKS(t *testing.T) {
	eksResourceDetector := detector{logger: zap.NewNop()}
	r, _, err := eksResourceDetector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, r.Attributes().Len(), "Resource object should be empty")
}

func TestEKSResourceDetection_ForCloudAccountID(t *testing.T) {
	tests := []struct {
		name           string
		ra             metadata.ResourceAttributesConfig
		expectedOutput map[string]any
		shouldError    bool
	}{
		{
			name: "Detects CloudAccountID when enabled",
			ra: metadata.ResourceAttributesConfig{
				CloudAccountID: metadata.ResourceAttributeConfig{Enabled: true},
			},
			expectedOutput: map[string]any{
				"cloud.account.id": "cloud1234",
			},
			shouldError: false,
		},
		{
			name: "Does not detect CloudAccountID when disabled",
			ra: metadata.ResourceAttributesConfig{
				CloudAccountID: metadata.ResourceAttributeConfig{Enabled: false},
			},
			expectedOutput: map[string]any{},
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detectorUtils := new(MockDetectorUtils)
			ctx := context.Background()

			t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
			detectorUtils.On("getConfigMap", authConfigmapNS, authConfigmapName).Return(map[string]string{conventions.AttributeK8SClusterName: clusterName}, nil)

			eksResourceDetector := &detector{
				utils: detectorUtils,
				err:   nil,
				ra:    tt.ra,
				rb:    metadata.NewResourceBuilder(tt.ra),
			}
			res, _, err := eksResourceDetector.Detect(ctx)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
		})
	}
}
