// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/resourcequota"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, rq *corev1.ResourceQuota, ts pcommon.Timestamp) {
	for k, v := range rq.Status.Hard {
		val := v.Value()
		if strings.HasSuffix(string(k), ".cpu") {
			val = v.MilliValue()
		}
		mb.RecordK8sResourceQuotaHardLimitDataPoint(ts, val, string(k))
	}

	for k, v := range rq.Status.Used {
		val := v.Value()
		if strings.HasSuffix(string(k), ".cpu") {
			val = v.MilliValue()
		}
		mb.RecordK8sResourceQuotaUsedDataPoint(ts, val, string(k))
	}

	rb := mb.NewResourceBuilder()
	rb.SetK8sResourcequotaUID(string(rq.UID))
	rb.SetK8sResourcequotaName(rq.Name)
	rb.SetK8sNamespaceName(rq.Namespace)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}
