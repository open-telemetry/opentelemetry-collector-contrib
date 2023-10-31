// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/namespace"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *imetadata.MetricsBuilder, ns *corev1.Namespace, ts pcommon.Timestamp) {
	mb.RecordK8sNamespacePhaseDataPoint(ts, int64(namespacePhaseValues[ns.Status.Phase]))
	rb := mb.NewResourceBuilder()
	rb.SetK8sNamespaceUID(string(ns.UID))
	rb.SetK8sNamespaceName(ns.Name)
	mb.EmitForResource(imetadata.WithResource(rb.Emit()))
}

var namespacePhaseValues = map[corev1.NamespacePhase]int32{
	corev1.NamespaceActive:      1,
	corev1.NamespaceTerminating: 0,
	// If phase is blank for some reason, send as -1 for unknown.
	corev1.NamespacePhase(""): -1,
}
