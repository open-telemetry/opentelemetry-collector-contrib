// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clusterresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota"

import (
	"strings"
	"time"

	quotav1 "github.com/openshift/api/quota/v1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, crq *quotav1.ClusterResourceQuota) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	for k, v := range crq.Status.Total.Hard {
		val := extractValue(k, v)
		mbphase.RecordOpenshiftClusterquotaLimitDataPoint(ts, val, string(k))
	}

	for k, v := range crq.Status.Total.Used {
		val := extractValue(k, v)
		mbphase.RecordOpenshiftClusterquotaUsedDataPoint(ts, val, string(k))
	}

	for _, ns := range crq.Status.Namespaces {
		for k, v := range ns.Status.Hard {
			val := extractValue(k, v)
			mbphase.RecordOpenshiftAppliedclusterquotaLimitDataPoint(ts, val, ns.Namespace, string(k))
		}

		for k, v := range ns.Status.Used {
			val := extractValue(k, v)
			mbphase.RecordOpenshiftAppliedclusterquotaUsedDataPoint(ts, val, ns.Namespace, string(k))
		}
	}

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetOpenshiftClusterquotaName(crq.Name)
	rb.SetOpenshiftClusterquotaUID(string(crq.UID))
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

func extractValue(k v1.ResourceName, v resource.Quantity) int64 {
	val := v.Value()
	if strings.HasSuffix(string(k), ".cpu") {
		val = v.MilliValue()
	}
	return val
}
