// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clusterresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota"

import (
	"strings"

	quotav1 "github.com/openshift/api/quota/v1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, crq *quotav1.ClusterResourceQuota, ts pcommon.Timestamp) {
	for k, v := range crq.Status.Total.Hard {
		val := extractValue(k, v)
		mb.RecordOpenshiftClusterquotaLimitDataPoint(ts, val, string(k))
	}

	for k, v := range crq.Status.Total.Used {
		val := extractValue(k, v)
		mb.RecordOpenshiftClusterquotaUsedDataPoint(ts, val, string(k))
	}

	for _, ns := range crq.Status.Namespaces {
		for k, v := range ns.Status.Hard {
			val := extractValue(k, v)
			mb.RecordOpenshiftAppliedclusterquotaLimitDataPoint(ts, val, ns.Namespace, string(k))
		}

		for k, v := range ns.Status.Used {
			val := extractValue(k, v)
			mb.RecordOpenshiftAppliedclusterquotaUsedDataPoint(ts, val, ns.Namespace, string(k))
		}
	}

	rb := mb.NewResourceBuilder()
	rb.SetOpenshiftClusterquotaName(crq.Name)
	rb.SetOpenshiftClusterquotaUID(string(crq.UID))
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func extractValue(k v1.ResourceName, v resource.Quantity) int64 {
	val := v.Value()
	if strings.HasSuffix(string(k), ".cpu") {
		val = v.MilliValue()
	}
	return val
}
