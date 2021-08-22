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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(componenttest.NewNopProcessorCreateSettings(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, os.Setenv("KUBERNETES_SERVICE_HOST", "localhost"))

	// Call EKS Resource detector to detect resources
	eksResourceDetector := &Detector{}
	res, _, err := eksResourceDetector.Detect(ctx)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider": "aws",
		"cloud.platform": "aws_eks",
	}, internal.AttributesToMap(res.Attributes()), "Resource object returned is incorrect")
}

// Tests EKS resource detector not running in EKS environment
func TestNotEKS(t *testing.T) {
	detector := Detector{}
	require.NoError(t, os.Unsetenv("KUBERNETES_SERVICE_HOST"))
	r, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, r.Attributes().Len(), "Resource object should be empty")
}
