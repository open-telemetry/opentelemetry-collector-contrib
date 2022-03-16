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
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHPAMetrics(t *testing.T) {
	hpa := newHPA("1")

	actualResourceMetrics := getMetricsForHPA(hpa)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.resource, k8sType,
		map[string]string{
			"k8s.hpa.uid":        "test-hpa-1-uid",
			"k8s.hpa.name":       "test-hpa-1",
			"k8s.namespace.name": "test-namespace",
			"k8s.cluster.name":   "test-cluster",
		},
	)

	testutils.AssertMetricsInt(t, rm.metrics[0], "k8s.hpa.max_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, rm.metrics[1], "k8s.hpa.min_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, rm.metrics[2], "k8s.hpa.current_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.metrics[3], "k8s.hpa.desired_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 7)
}

func newHPA(id string) *autoscalingv2beta2.HorizontalPodAutoscaler {
	minReplicas := int32(2)
	return &autoscalingv2beta2.HorizontalPodAutoscaler{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-hpa-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-hpa-" + id + "-uid"),
			ClusterName: "test-cluster",
		},
		Status: autoscalingv2beta2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 5,
			DesiredReplicas: 7,
		},
		Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: 10,
		},
	}
}
