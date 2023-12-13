// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hierarchicalresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hierarchicalresourcequota"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, hrq *v1alpha2.HierarchicalResourceQuota, ts pcommon.Timestamp) {
	for k, v := range hrq.Status.Hard {
		val := parseQuantityValue(k, v)
		mb.RecordK8sHierarchicalResourceQuotaHardLimitDataPoint(ts, val, string(k))
	}
	for k, v := range hrq.Status.Used {
		val := parseQuantityValue(k, v)
		mb.RecordK8sHierarchicalResourceQuotaUsedDataPoint(ts, val, string(k))
	}
	rb := mb.NewResourceBuilder()
	rb.SetK8sHierarchicalresourcequotaUID(string(hrq.UID))
	rb.SetK8sHierarchicalresourcequotaName(hrq.Name)
	rb.SetK8sNamespaceName(hrq.Namespace)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func Transform(unstructedHRQ *unstructured.Unstructured) (*v1alpha2.HierarchicalResourceQuota, error) {
	name, found, err := unstructured.NestedString(unstructedHRQ.Object, "metadata", "name")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("not found HierarchicalResourceQuota %s property", "name")
	}
	uid, found, err := unstructured.NestedString(unstructedHRQ.Object, "metadata", "uid")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("not found HierarchicalResourceQuota %s property", "uid")
	}
	namespace, found, err := unstructured.NestedString(unstructedHRQ.Object, "metadata", "namespace")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("not found HierarchicalResourceQuota %s property", "namespace")
	}

	hrqs := v1alpha2.HierarchicalResourceQuotaStatus{
		Hard: corev1.ResourceList{},
		Used: corev1.ResourceList{},
	}
	statusHard, found, err := unstructured.NestedMap(unstructedHRQ.Object, "status", "hard")
	if err != nil {
		return nil, err
	} else if found {
		for k, v := range statusHard {
			hrqs.Hard[corev1.ResourceName(k)] = resource.MustParse(v.(string))
		}
	}
	statusUsed, found, err := unstructured.NestedMap(unstructedHRQ.Object, "status", "used")
	if err != nil {
		return nil, err
	} else if found {
		for k, v := range statusUsed {
			hrqs.Used[corev1.ResourceName(k)] = resource.MustParse(v.(string))
		}
	}
	hrq := &v1alpha2.HierarchicalResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Status: hrqs,
	}
	return hrq, nil
}

func parseQuantityValue(k corev1.ResourceName, v resource.Quantity) int64 {
	if strings.HasSuffix(k.String(), ".cpu") {
		return v.MilliValue()
	}
	return v.Value()
}
