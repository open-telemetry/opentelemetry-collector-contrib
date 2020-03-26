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

package kubernetesclusterreceiver

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opencensus.io/resource/resourcekeys"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubernetesclusterreceiver/utils"
)

var podsActiveMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/job/active_pods",
	Description: "The number of actively running pods for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsDesiredCompletedMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/job/desired_successful_pods",
	Description: "The desired number of successfully finished pods the job should be run with",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsFailedMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/job/failed_pods",
	Description: "The number of pods which reached phase Failed for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsMaxParallelMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/job/max_parallel_pods",
	Description: "The max desired number of pods the job should run at any given time",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var podsSuccessfulMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/job/successful_pods",
	Description: "The number of pods which reached phase Succeeded for a job",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForJob(j *batchv1.Job) []*resourceMetrics {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: podsActiveMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Active)),
			},
		},
		{
			MetricDescriptor: podsDesiredCompletedMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*j.Spec.Completions)),
			},
		},
		{
			MetricDescriptor: podsFailedMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Failed)),
			},
		},
		{
			MetricDescriptor: podsMaxParallelMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*j.Spec.Parallelism)),
			},
		},
		{
			MetricDescriptor: podsSuccessfulMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(j.Status.Succeeded)),
			},
		},
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
		Type: resourcekeys.K8SType,
		Labels: map[string]string{
			k8sKeyJobUID:                     string(j.UID),
			k8sKeyJobName:                    j.Name,
			resourcekeys.K8SKeyNamespaceName: j.Namespace,
			resourcekeys.K8SKeyClusterName:   j.ClusterName,
		},
	}
}

func getPropertiesForJob(cj *batchv1.Job) []*KubernetesMetadata {
	return []*KubernetesMetadata{getGenericMetadata(&cj.ObjectMeta, "job")}
}
