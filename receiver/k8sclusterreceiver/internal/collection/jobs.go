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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	batchv1 "k8s.io/api/batch/v1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

var podsActiveMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.job.active_pods",
	Description: "The number of actively running pods for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsDesiredCompletedMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.job.desired_successful_pods",
	Description: "The desired number of successfully finished pods the job should be run with",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsFailedMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.job.failed_pods",
	Description: "The number of pods which reached phase Failed for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsMaxParallelMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.job.max_parallel_pods",
	Description: "The max desired number of pods the job should run at any given time",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsSuccessfulMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.job.successful_pods",
	Description: "The number of pods which reached phase Succeeded for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForJob(j *batchv1.Job) []*resourceMetrics {
	metrics := make([]*metricspb.Metric, 0, 5)
	metrics = append(metrics, []*metricspb.Metric{
		{
			MetricDescriptor: podsActiveMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Active)),
			},
		},
		{
			MetricDescriptor: podsFailedMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Failed)),
			},
		},
		{
			MetricDescriptor: podsSuccessfulMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Succeeded)),
			},
		},
	}...)

	if j.Spec.Completions != nil {
		metrics = append(metrics, &metricspb.Metric{
			MetricDescriptor: podsDesiredCompletedMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*j.Spec.Completions)),
			}})
	}

	if j.Spec.Parallelism != nil {
		metrics = append(metrics, &metricspb.Metric{
			MetricDescriptor: podsMaxParallelMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*j.Spec.Parallelism)),
			}})
	}

	return []*resourceMetrics{
		{
			resource: getResourceForJob(j),
			metrics:  metrics,
		},
	}
}

func getResourceForJob(j *batchv1.Job) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8SJobUID:        string(j.UID),
			conventions.AttributeK8SJobName:       j.Name,
			conventions.AttributeK8SNamespaceName: j.Namespace,
			conventions.AttributeK8SClusterName:   j.ClusterName,
		},
	}
}

func getMetadataForJob(j *batchv1.Job) map[metadata.ResourceID]*KubernetesMetadata {
	return map[metadata.ResourceID]*KubernetesMetadata{
		metadata.ResourceID(j.UID): getGenericMetadata(&j.ObjectMeta, k8sKindJob),
	}
}
