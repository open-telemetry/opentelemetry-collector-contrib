// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jobs // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, j *batchv1.Job) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	mbphase.RecordK8sJobActivePodsDataPoint(ts, int64(j.Status.Active))
	mbphase.RecordK8sJobFailedPodsDataPoint(ts, int64(j.Status.Failed))
	mbphase.RecordK8sJobSuccessfulPodsDataPoint(ts, int64(j.Status.Succeeded))

	if j.Spec.Completions != nil {
		mbphase.RecordK8sJobDesiredSuccessfulPodsDataPoint(ts, int64(*j.Spec.Completions))
	}
	if j.Spec.Parallelism != nil {
		mbphase.RecordK8sJobMaxParallelPodsDataPoint(ts, int64(*j.Spec.Parallelism))
	}

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(j.Namespace)
	rb.SetK8sJobName(j.Name)
	rb.SetK8sJobUID(string(j.UID))
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

// Transform transforms the job to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new job fields.
func Transform(job *batchv1.Job) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metadata.TransformObjectMeta(job.ObjectMeta),
		Spec: batchv1.JobSpec{
			Completions: job.Spec.Completions,
			Parallelism: job.Spec.Parallelism,
		},
		Status: batchv1.JobStatus{
			Active:    job.Status.Active,
			Succeeded: job.Status.Succeeded,
			Failed:    job.Status.Failed,
		},
	}
}

func GetMetadata(j *batchv1.Job) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(j.UID): metadata.GetGenericMetadata(&j.ObjectMeta, constants.K8sKindJob),
	}
}
