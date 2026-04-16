// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jobs // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, j *batchv1.Job, ts pcommon.Timestamp) {
	e := metadata.NewK8sJobEntity(string(j.UID))
	e.SetK8sJobName(j.Name)
	e.SetK8sNamespaceName(j.Namespace)
	eb := mb.ForK8sJob(e)
	eb.RecordK8sJobActivePodsDataPoint(ts, int64(j.Status.Active))
	eb.RecordK8sJobFailedPodsDataPoint(ts, int64(j.Status.Failed))
	eb.RecordK8sJobSuccessfulPodsDataPoint(ts, int64(j.Status.Succeeded))

	if j.Spec.Completions != nil {
		eb.RecordK8sJobDesiredSuccessfulPodsDataPoint(ts, int64(*j.Spec.Completions))
	}
	if j.Spec.Parallelism != nil {
		eb.RecordK8sJobMaxParallelPodsDataPoint(ts, int64(*j.Spec.Parallelism))
	}
	eb.Emit()
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
