// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jobs // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
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

func GetMetrics(j *batchv1.Job) []*agentmetricspb.ExportMetricsServiceRequest {
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

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(j),
			Metrics:  metrics,
		},
	}
}

func getResource(j *batchv1.Job) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SJobUID:        string(j.UID),
			conventions.AttributeK8SJobName:       j.Name,
			conventions.AttributeK8SNamespaceName: j.Namespace,
		},
	}
}

func GetMetadata(j *batchv1.Job) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(j.UID): metadata.GetGenericMetadata(&j.ObjectMeta, constants.K8sKindJob),
	}
}
