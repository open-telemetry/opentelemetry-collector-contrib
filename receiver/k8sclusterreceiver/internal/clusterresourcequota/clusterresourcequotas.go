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

	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, crq *quotav1.ClusterResourceQuota) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	emitCRQMetrics(mbphase, crq, crq.Status.Total.Hard, "openshift.clusterquota.limit", ts, "")
	emitCRQMetrics(mbphase, crq, crq.Status.Total.Used, "openshift.clusterquota.used", ts, "")

	for _, ns := range crq.Status.Namespaces {
		emitCRQMetrics(mbphase, crq, ns.Status.Hard, "openshift.appliedclusterquota.limit", ts, ns.Namespace)
		emitCRQMetrics(mbphase, crq, ns.Status.Hard, "openshift.appliedclusterquota.used", ts, ns.Namespace)

	}

	return mbphase.Emit()
}

func emitCRQMetrics(mbphase *imetadataphase.MetricsBuilder, crq *quotav1.ClusterResourceQuota, rl v1.ResourceList, metricName string, ts pcommon.Timestamp, namespace string) {
	for k, v := range rl {
		val := v.Value()
		if strings.HasSuffix(string(k), ".cpu") {
			val = v.MilliValue()
		}

		switch metricName {
		case "openshift.clusterquota.limit":
			mbphase.RecordOpenshiftClusterquotaLimitDataPoint(ts, val)
		case "openshift.clusterquota.used":
			mbphase.RecordOpenshiftClusterquotaUsedDataPoint(ts, val)
		case "openshift.appliedclusterquota.limit":
			mbphase.RecordOpenshiftAppliedclusterquotaLimitDataPoint(ts, val)
		case "openshift.appliedclusterquota.used":
			mbphase.RecordOpenshiftAppliedclusterquotaUsedDataPoint(ts, val)
		default:
			return
		}

		if namespace != "" {
			mbphase.EmitForResource(imetadataphase.WithK8sNamespaceName(namespace), imetadataphase.WithOpenshiftClusterquotaUID(crq.Name), imetadataphase.WithOpenshiftClusterquotaName(string(crq.UID)), imetadataphase.WithOpencensusResourcetype(string(k)))
		} else {
			mbphase.EmitForResource(imetadataphase.WithOpenshiftClusterquotaUID(crq.Name), imetadataphase.WithOpenshiftClusterquotaName(string(crq.UID)), imetadataphase.WithOpencensusResourcetype(string(k)))
		}

	}
}
