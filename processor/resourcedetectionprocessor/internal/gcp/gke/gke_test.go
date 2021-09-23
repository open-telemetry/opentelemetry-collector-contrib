// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gke

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
)

func TestNotGCE(t *testing.T) {
	metadata := &gcp.MockMetadata{}
	detector := &Detector{
		log:      zap.NewNop(),
		metadata: metadata,
	}

	metadata.On("OnGCE").Return(false)
	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())

	metadata.AssertExpectations(t)
}

func TestDetectWithoutCluster(t *testing.T) {
	metadata := &gcp.MockMetadata{}
	detector := &Detector{
		log:      zap.NewNop(),
		metadata: metadata,
	}

	metadata.On("OnGCE").Return(true)
	metadata.On("InstanceAttributeValue", "cluster-name").Return("", errors.New("no cluster"))

	require.NoError(t, os.Setenv("KUBERNETES_SERVICE_HOST", "localhost"))

	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider": "gcp",
		"cloud.platform": "gcp_kubernetes_engine",
	}, internal.AttributesToMap(res.Attributes()))

	metadata.AssertExpectations(t)
}

func TestDetectWithoutK8s(t *testing.T) {
	metadata := &gcp.MockMetadata{}
	detector := &Detector{
		log:      zap.NewNop(),
		metadata: metadata,
	}

	metadata.On("OnGCE").Return(true)

	require.NoError(t, os.Unsetenv("KUBERNETES_SERVICE_HOST"))

	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider": "gcp",
	}, internal.AttributesToMap(res.Attributes()))

	metadata.AssertExpectations(t)
}

func TestDetector_Detect(t *testing.T) {
	metadata := &gcp.MockMetadata{}
	detector := &Detector{
		log:      zap.NewNop(),
		metadata: metadata,
	}

	metadata.On("OnGCE").Return(true)
	metadata.On("InstanceAttributeValue", "cluster-name").Return("cluster-a", nil)

	require.NoError(t, os.Setenv("KUBERNETES_SERVICE_HOST", "localhost"))

	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider":   "gcp",
		"cloud.platform":   "gcp_kubernetes_engine",
		"k8s.cluster.name": "cluster-a",
	}, internal.AttributesToMap(res.Attributes()))

	metadata.AssertExpectations(t)
}

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(componenttest.NewNopProcessorCreateSettings(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}
