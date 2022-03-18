// Copyright 2020 OpenTelemetry Authors
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

package collection

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestStatefulsettMetrics(t *testing.T) {
	ss := newStatefulset("1")

	actualResourceMetrics := getMetricsForStatefulSet(ss)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.resource, k8sType,
		map[string]string{
			"k8s.statefulset.uid":  "test-statefulset-1-uid",
			"k8s.statefulset.name": "test-statefulset-1",
			"k8s.namespace.name":   "test-namespace",
			"k8s.cluster.name":     "test-cluster",
		},
	)

	testutils.AssertMetricsInt(t, rm.metrics[0], "k8s.statefulset.desired_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, rm.metrics[1], "k8s.statefulset.ready_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 7)

	testutils.AssertMetricsInt(t, rm.metrics[2], "k8s.statefulset.current_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.metrics[3], "k8s.statefulset.updated_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)
}

func TestStatefulsetMetadata(t *testing.T) {
	ss := newStatefulset("1")

	actualMetadata := getMetadataForStatefulSet(ss)

	require.Equal(t, 1, len(actualMetadata))

	require.Equal(t,
		KubernetesMetadata{
			resourceIDKey: "k8s.statefulset.uid",
			resourceID:    "test-statefulset-1-uid",
			metadata: map[string]string{
				"k8s.workload.name":              "test-statefulset-1",
				"k8s.workload.kind":              "StatefulSet",
				"statefulset.creation_timestamp": "0001-01-01T00:00:00Z",
				"foo":                            "bar",
				"foo1":                           "",
				"current_revision":               "current_revision",
				"update_revision":                "update_revision",
			},
		},
		*actualMetadata["test-statefulset-1-uid"],
	)
}

func newStatefulset(id string) *appsv1.StatefulSet {
	desired := int32(10)
	return &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-statefulset-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-statefulset-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &desired,
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   7,
			CurrentReplicas: 5,
			UpdatedReplicas: 3,
			CurrentRevision: "current_revision",
			UpdateRevision:  "update_revision",
		},
	}
}
