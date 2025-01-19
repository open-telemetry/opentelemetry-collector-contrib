// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cronjob // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for cronjob metadata.
	cronJobKeySchedule          = "schedule"
	cronJobKeyConcurrencyPolicy = "concurrency_policy"
)

func RecordMetrics(mb *metadata.MetricsBuilder, cj *batchv1.CronJob, ts pcommon.Timestamp) {
	mb.RecordK8sCronjobActiveJobsDataPoint(ts, int64(len(cj.Status.Active)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sNamespaceName(cj.Namespace)
	rb.SetK8sCronjobUID(string(cj.UID))
	rb.SetK8sCronjobName(cj.Name)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func GetMetadata(cj *batchv1.CronJob) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&cj.ObjectMeta, constants.K8sKindCronJob)
	rm.Metadata[cronJobKeySchedule] = cj.Spec.Schedule
	rm.Metadata[cronJobKeyConcurrencyPolicy] = string(cj.Spec.ConcurrencyPolicy)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(cj.UID): rm}
}
