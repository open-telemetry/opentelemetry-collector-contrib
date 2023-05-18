// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statefulset

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestStatefulsettMetrics(t *testing.T) {
	ss := newStatefulset("1")

	actualResourceMetrics := GetMetrics(ss)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.statefulset.uid":  "test-statefulset-1-uid",
			"k8s.statefulset.name": "test-statefulset-1",
			"k8s.namespace.name":   "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.statefulset.desired_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.statefulset.ready_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 7)

	testutils.AssertMetricsInt(t, rm.Metrics[2], "k8s.statefulset.current_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.Metrics[3], "k8s.statefulset.updated_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)
}

func TestStatefulsetMetadata(t *testing.T) {
	ss := newStatefulset("1")

	actualMetadata := GetMetadata(ss)

	require.Equal(t, 1, len(actualMetadata))

	require.Equal(t,
		metadata.KubernetesMetadata{
			ResourceIDKey: "k8s.statefulset.uid",
			ResourceID:    "test-statefulset-1-uid",
			Metadata: map[string]string{
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
			Name:      "test-statefulset-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-statefulset-" + id + "-uid"),
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
