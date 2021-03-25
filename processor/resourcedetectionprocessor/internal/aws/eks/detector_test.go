// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type MockDetectorUtils struct {
	mock.Mock
}

// Mock function for fileExists()
func (detectorUtils *MockDetectorUtils) fileExists(filename string) bool {
	args := detectorUtils.Called(filename)
	return args.Bool(0)
}

// Mock function for fetchString()
func (detectorUtils *MockDetectorUtils) fetchString(ctx context.Context, httpMethod string, URL string) (string, error) {
	args := detectorUtils.Called(ctx, httpMethod, URL)
	return args.String(0), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(component.ProcessorCreateParams{Logger: zap.NewNop()}, nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}

// Tests EKS resource detector running in EKS environment
func TestEks(t *testing.T) {
	detectorUtils := &MockDetectorUtils{}
	ctx := context.Background()

	// Mock functions and set expectations
	detectorUtils.On("fileExists", k8sTokenPath).Return(true)
	detectorUtils.On("fileExists", k8sCertPath).Return(true)
	detectorUtils.On("fetchString", ctx, "GET", authConfigmapPath).Return("not empty", nil)
	detectorUtils.On("fetchString", ctx, "GET", cwConfigmapPath).Return(`{"data":{"cluster.name":"my-cluster"}}`, nil)

	// Call EKS Resource detector to detect resources
	eksResourceDetector := &Detector{utils: detectorUtils}
	res, err := eksResourceDetector.Detect(ctx)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider":               "aws",
		"cloud.infrastructure_service": "aws_eks",
		"k8s.cluster.name":             "my-cluster",
	}, internal.AttributesToMap(res.Attributes()), "Resource object returned is incorrect")
	detectorUtils.AssertExpectations(t)
}

// Tests EKS resource detector not running in EKS environment
func TestNotEKS(t *testing.T) {
	detectorUtils := new(MockDetectorUtils)

	/* #nosec */
	k8sTokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// Mock functions and set expectations
	detectorUtils.On("fileExists", k8sTokenPath).Return(false)

	detector := Detector{detectorUtils}
	r, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, r.Attributes().Len(), "Resource object should be empty")
	detectorUtils.AssertExpectations(t)
}
