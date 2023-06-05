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

package cronjob

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestCronJobMetrics(t *testing.T) {
	cj := newCronJob("1")

	actualResourceMetrics := GetMetrics(cj)

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 1, len(actualResourceMetrics[0].Metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.cronjob.uid":    "test-cronjob-1-uid",
			"k8s.cronjob.name":   "test-cronjob-1",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.cronjob.active_jobs",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}

func TestCronJobMetadata(t *testing.T) {
	cj := newCronJob("1")

	actualMetadata := GetMetadata(cj)

	require.Equal(t, 1, len(actualMetadata))

	// Assert metadata from Pod.
	require.Equal(t,
		metadata.KubernetesMetadata{
			ResourceIDKey: "k8s.cronjob.uid",
			ResourceID:    "test-cronjob-1-uid",
			Metadata: map[string]string{
				"cronjob.creation_timestamp": "0001-01-01T00:00:00Z",
				"foo":                        "bar",
				"foo1":                       "",
				"schedule":                   "schedule",
				"concurrency_policy":         "concurrency_policy",
				"k8s.workload.kind":          "CronJob",
				"k8s.workload.name":          "test-cronjob-1",
			},
		},
		*actualMetadata["test-cronjob-1-uid"],
	)
}

func newCronJob(id string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-cronjob-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-cronjob-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          "schedule",
			ConcurrencyPolicy: "concurrency_policy",
		},
		Status: batchv1.CronJobStatus{
			Active: []corev1.ObjectReference{{}, {}},
		},
	}
}
