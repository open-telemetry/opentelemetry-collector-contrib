// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/resourcequota"

import (
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	corev1 "k8s.io/api/core/v1"

	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/resourcequota/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, rq *corev1.ResourceQuota) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	for _, t := range []struct {
		recordFn func(ts pcommon.Timestamp, val int64, resource string)
		rl       corev1.ResourceList
	}{
		{
			mb.RecordK8sResourceQuotaHardLimitDataPoint,
			rq.Status.Hard,
		},
		{
			mb.RecordK8sResourceQuotaUsedDataPoint,
			rq.Status.Used,
		},
	} {
		for k, v := range t.rl {

			val := v.Value()
			if strings.HasSuffix(string(k), ".cpu") {
				val = v.MilliValue()
			}
			mb.RecordK8sResourceQuotaUsedDataPoint(ts, val, string(k))
			mb.RecordK8sResourceQuotaHardLimitDataPoint(ts, val, string(k))
		}
	}

	return mb.Emit(imetadata.WithK8sResourcequotaUID(string(rq.UID)), imetadata.WithK8sResourcequotaName(rq.Name), imetadata.WithK8sNamespaceName(rq.Namespace))
}
