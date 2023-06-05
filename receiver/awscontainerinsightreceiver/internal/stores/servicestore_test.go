// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stores

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type mockEndpoint struct {
}

func (m *mockEndpoint) PodKeyToServiceNames() map[string][]string {
	return map[string][]string{
		"namespace:default,podName:test-pod": {"test-service"},
	}
}

func TestServiceStore(t *testing.T) {
	s := &ServiceStore{
		podKeyToServiceNamesMap: make(map[string][]string),
		logger:                  zap.NewNop(),
		endpointInfo:            &mockEndpoint{},
	}

	ctx := context.Background()
	s.lastRefreshed = time.Now().Add(-20 * time.Second)
	s.RefreshTick(ctx)

	// test the case when it decorates metrics successfully
	metric := &mockCIMetric{
		tags: map[string]string{
			ci.K8sPodNameKey: "test-pod",
			ci.K8sNamespace:  "default",
		},
	}
	kubernetesBlob := map[string]interface{}{}
	ok := s.Decorate(ctx, metric, kubernetesBlob)
	assert.True(t, ok)
	assert.Equal(t, "test-service", metric.GetTag(ci.TypeService))

	// test the case when it fails to decorate metrics
	metric = &mockCIMetric{
		tags: map[string]string{
			ci.K8sPodNameKey: "test-pod",
		},
	}
	kubernetesBlob = map[string]interface{}{}
	ok = s.Decorate(ctx, metric, kubernetesBlob)
	assert.False(t, ok)
	assert.Equal(t, "", metric.GetTag(ci.TypeService))
}
