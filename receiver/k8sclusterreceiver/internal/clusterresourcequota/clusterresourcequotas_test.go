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

package clusterresourcequota

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestClusterRequestQuotaMetrics(t *testing.T) {
	rq := newMockClusterResourceQuota("1")

	actualResourceMetrics := GetMetrics(rq)

	require.Equal(t, 1, len(actualResourceMetrics))

	metrics := actualResourceMetrics[0].Metrics
	require.Equal(t, 6, len(metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"openshift.clusterquota.uid":  "test-clusterquota-1-uid",
			"openshift.clusterquota.name": "test-clusterquota-1",
		},
	)

	for i, tc := range []struct {
		name   string
		value  int64
		labels map[string]string
	}{
		{
			"openshift.clusterquota.limit",
			10000,
			map[string]string{
				"resource": "requests.cpu",
			},
		},
		{
			"openshift.clusterquota.used",
			6000,
			map[string]string{
				"resource": "requests.cpu",
			},
		},
		{
			"openshift.appliedclusterquota.limit",
			6000,
			map[string]string{
				"resource":           "requests.cpu",
				"k8s.namespace.name": "ns1",
			},
		},
		{
			"openshift.appliedclusterquota.used",
			1000,
			map[string]string{
				"resource":           "requests.cpu",
				"k8s.namespace.name": "ns1",
			},
		},
		{
			"openshift.appliedclusterquota.limit",
			4000,
			map[string]string{
				"resource":           "requests.cpu",
				"k8s.namespace.name": "ns2",
			},
		},
		{
			"openshift.appliedclusterquota.used",
			5000,
			map[string]string{
				"resource":           "requests.cpu",
				"k8s.namespace.name": "ns2",
			},
		},
	} {
		testutils.AssertMetricsWithLabels(t, metrics[i], tc.name,
			metricspb.MetricDescriptor_GAUGE_INT64, tc.labels, tc.value)
	}
}

func newMockClusterResourceQuota(id string) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-clusterquota-" + id,
			UID:  types.UID("test-clusterquota-" + id + "-uid"),
		},
		Status: quotav1.ClusterResourceQuotaStatus{
			Total: corev1.ResourceQuotaStatus{
				Hard: corev1.ResourceList{
					"requests.cpu": *resource.NewQuantity(10, resource.DecimalSI),
				},
				Used: corev1.ResourceList{
					"requests.cpu": *resource.NewQuantity(6, resource.DecimalSI),
				},
			},
			Namespaces: quotav1.ResourceQuotasStatusByNamespace{
				{
					Namespace: "ns1",
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							"requests.cpu": *resource.NewQuantity(6, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							"requests.cpu": *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
				{
					Namespace: "ns2",
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							"requests.cpu": *resource.NewQuantity(4, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							"requests.cpu": *resource.NewQuantity(5, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
}
