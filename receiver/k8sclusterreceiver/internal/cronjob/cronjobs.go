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

package cronjob // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
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

// TODO: All the CronJob related functions below can be de-duplicated using generics from go 1.19

func GetMetrics(cj *batchv1.CronJob) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: activeJobs,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(len(cj.Status.Active))),
			},
		},
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResourceForCronJob(cj),
			Metrics:  metrics,
		},
	}
}

func GetMetricsBeta(cj *batchv1beta1.CronJob) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: activeJobs,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(len(cj.Status.Active))),
			},
		},
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResourceForCronJobBeta(cj),
			Metrics:  metrics,
		},
	}
}

func getResourceForCronJob(cj *batchv1.CronJob) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SCronJobUID:    string(cj.UID),
			conventions.AttributeK8SCronJobName:   cj.Name,
			conventions.AttributeK8SNamespaceName: cj.Namespace,
		},
	}
}

func getResourceForCronJobBeta(cj *batchv1beta1.CronJob) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SCronJobUID:    string(cj.UID),
			conventions.AttributeK8SCronJobName:   cj.Name,
			conventions.AttributeK8SNamespaceName: cj.Namespace,
		},
	}
}

func GetMetadata(cj *batchv1.CronJob) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&cj.ObjectMeta, constants.K8sKindCronJob)
	rm.Metadata[cronJobKeySchedule] = cj.Spec.Schedule
	rm.Metadata[cronJobKeyConcurrencyPolicy] = string(cj.Spec.ConcurrencyPolicy)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(cj.UID): rm}
}

func GetMetadataBeta(cj *batchv1beta1.CronJob) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&cj.ObjectMeta, constants.K8sKindCronJob)
	rm.Metadata[cronJobKeySchedule] = cj.Spec.Schedule
	rm.Metadata[cronJobKeyConcurrencyPolicy] = string(cj.Spec.ConcurrencyPolicy)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(cj.UID): rm}
}
