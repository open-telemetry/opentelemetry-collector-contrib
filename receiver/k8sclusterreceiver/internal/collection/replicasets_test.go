// Copyright 2020, OpenTelemetry Authors
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

func TestReplicasetMetrics(t *testing.T) {
	rs := newReplicaSet("1")

	actualResourceMetrics := getMetricsForReplicaSet(rs)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 2, len(actualResourceMetrics[0].metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.resource, k8sType,
		map[string]string{
			"k8s.replicaset.uid":  "test-replicaset-1-uid",
			"k8s.replicaset.name": "test-replicaset-1",
			"k8s.namespace.name":  "test-namespace",
			"k8s.cluster.name":    "test-cluster",
		},
	)

	testutils.AssertMetricsInt(t, rm.metrics[0], "k8s.replicaset.desired",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, rm.metrics[1], "k8s.replicaset.available",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}

func newReplicaSet(id string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-replicaset-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-replicaset-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: func() *int32 {
				var out int32 = 3
				return &out
			}(),
		},
		Status: appsv1.ReplicaSetStatus{
			AvailableReplicas: 2,
		},
	}
}
