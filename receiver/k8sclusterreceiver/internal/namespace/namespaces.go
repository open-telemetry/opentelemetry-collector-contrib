// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/namespace"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	corev1 "k8s.io/api/core/v1"

	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/namespace/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, ns *corev1.Namespace) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mb.RecordK8sNamespacePhaseDataPoint(ts, int64(namespacePhaseValues[ns.Status.Phase]))
	rb := imetadata.NewResourceBuilder(imetadata.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceUID(string(ns.UID))
	rb.SetK8sNamespaceName(ns.Name)
	rb.SetOpencensusResourcetype("k8s")
	return mb.Emit(imetadata.WithResource(rb.Emit()))
}

var namespacePhaseValues = map[corev1.NamespacePhase]int32{
	corev1.NamespaceActive:      1,
	corev1.NamespaceTerminating: 0,
	// If phase is blank for some reason, send as -1 for unknown.
	corev1.NamespacePhase(""): -1,
}
