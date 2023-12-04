// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hierarchicalresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hierarchicalresourcequota"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, hrq *unstructured.Unstructured, ts pcommon.Timestamp) {
	name, _, _ := unstructured.NestedString(hrq.Object, "metadata", "name")
	uid, _, _ := unstructured.NestedString(hrq.Object, "metadata", "uid")
	namespace, _, _ := unstructured.NestedString(hrq.Object, "metadata", "namespace")
	statusHard, _, _ := unstructured.NestedMap(hrq.Object, "status", "hard")
	for k, v := range statusHard {
		val := parseQuantityValue(k, v.(string))
		mb.RecordK8sHierarchicalResourceQuotaHardLimitDataPoint(ts, val, k)
	}
	statusUsed, _, _ := unstructured.NestedMap(hrq.Object, "status", "used")
	for k, v := range statusUsed {
		val := parseQuantityValue(k, v.(string))
		mb.RecordK8sHierarchicalResourceQuotaUsedDataPoint(ts, val, k)
	}

	rb := mb.NewResourceBuilder()
	rb.SetK8sHierarchicalresourcequotaUID(uid)
	rb.SetK8sHierarchicalresourcequotaName(name)
	rb.SetK8sNamespaceName(namespace)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func parseQuantityValue(k string, v string) int64 {
	q := resource.MustParse(v)
	if strings.HasSuffix(k, ".cpu") {
		return q.MilliValue()
	}
	return q.Value()
}
