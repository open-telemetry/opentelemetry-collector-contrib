// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cronjob // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for cronjob metadata.
	cronJobKeySchedule          = "schedule"
	cronJobKeyConcurrencyPolicy = "concurrency_policy"
)

func GetMetrics(set receiver.CreateSettings, cj *batchv1.CronJob) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	mbphase.RecordK8sCronjobActiveJobsDataPoint(ts, int64(len(cj.Status.Active)))

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(cj.Namespace)
	rb.SetK8sCronjobUID(string(cj.UID))
	rb.SetK8sCronjobName(cj.Name)
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

func GetMetricsBeta(set receiver.CreateSettings, cj *batchv1beta1.CronJob) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	mbphase.RecordK8sCronjobActiveJobsDataPoint(ts, int64(len(cj.Status.Active)))

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(cj.Namespace)
	rb.SetK8sCronjobUID(string(cj.UID))
	rb.SetK8sCronjobName(cj.Name)
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
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
