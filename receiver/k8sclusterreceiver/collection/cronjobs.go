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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/translator/conventions"
	batchv1beta1 "k8s.io/api/batch/v1beta1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for cronjob metadata.
	cronJobKeySchedule          = "schedule"
	cronJobKeyConcurrencyPolicy = "concurrency_policy"
)

var activeJobs = &metricspb.MetricDescriptor{
	Name:        "k8s.cronjob.active_jobs",
	Description: "The number of actively running jobs for a cronjob",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForCronJob(cj *batchv1beta1.CronJob) []*resourceMetrics {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: activeJobs,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(len(cj.Status.Active))),
			},
		},
	}

	return []*resourceMetrics{
		{
			resource: getResourceForCronJob(cj),
			metrics:  metrics,
		},
	}
}

func getResourceForCronJob(cj *batchv1beta1.CronJob) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8sCronJobUID: string(cj.UID),
			conventions.AttributeK8sCronJob:    cj.Name,
			conventions.AttributeK8sNamespace:  cj.Namespace,
			conventions.AttributeK8sCluster:    cj.ClusterName,
		},
	}
}

func getMetadataForCronJob(cj *batchv1beta1.CronJob) map[metadata.ResourceID]*KubernetesMetadata {
	rm := getGenericMetadata(&cj.ObjectMeta, k8sKindCronJob)
	rm.metadata[cronJobKeySchedule] = cj.Spec.Schedule
	rm.metadata[cronJobKeyConcurrencyPolicy] = string(cj.Spec.ConcurrencyPolicy)
	return map[metadata.ResourceID]*KubernetesMetadata{metadata.ResourceID(cj.UID): rm}
}
