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

package resourcequota

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestRequestQuotaMetrics(t *testing.T) {
	rq := newResourceQuota("1")

	actualResourceMetrics := GetMetrics(rq)

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 2, len(actualResourceMetrics[0].Metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.resourcequota.uid":  "test-resourcequota-1-uid",
			"k8s.resourcequota.name": "test-resourcequota-1",
			"k8s.namespace.name":     "test-namespace",
		},
	)

	testutils.AssertMetricsWithLabels(t, actualResourceMetrics[0].Metrics[0], "k8s.resource_quota.hard_limit",
		metricspb.MetricDescriptor_GAUGE_INT64, map[string]string{"resource": "requests.cpu"}, 2000)

	testutils.AssertMetricsWithLabels(t, actualResourceMetrics[0].Metrics[1], "k8s.resource_quota.used",
		metricspb.MetricDescriptor_GAUGE_INT64, map[string]string{"resource": "requests.cpu"}, 1000)
}

func newResourceQuota(id string) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-resourcequota-" + id,
			UID:       types.UID("test-resourcequota-" + id + "-uid"),
			Namespace: "test-namespace",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{
				"requests.cpu": *resource.NewQuantity(2, resource.DecimalSI),
			},
			Used: corev1.ResourceList{
				"requests.cpu": *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}
}
