// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hierarchicalresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hierarchicalresourcequota"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(logger *zap.Logger, mb *metadata.MetricsBuilder, hrq *unstructured.Unstructured, ts pcommon.Timestamp) {
	name, found, err := unstructured.NestedString(hrq.Object, "metadata", "name")
	if err != nil {
		logger.Error("Failed to get HierarchicalResourceQuota name property.", zap.Error(err))
		return
	} else if !found {
		logger.Error("Not found HierarchicalResourceQuota name property.")
		return
	}
	// uid and namespace are optional properties.
	uid, found, err := unstructured.NestedString(hrq.Object, "metadata", "uid")
	if err != nil {
		logger.Debug("Failed to get HierarchicalResourceQuota uid property.", zap.Error(err))
	} else if !found {
		logger.Debug("Not found HierarchicalResourceQuota uid property.")
	}
	namespace, found, err := unstructured.NestedString(hrq.Object, "metadata", "namespace")
	if err != nil {
		logger.Debug("Failed to get HierarchicalResourceQuota namespace property.", zap.Error(err))
	} else if !found {
		logger.Debug("Not found HierarchicalResourceQuota namespace property.")
	}

	statusHard, found, err := unstructured.NestedMap(hrq.Object, "status", "hard")
	if err != nil {
		logger.Warn("Failed to get HierarchicalResourceQuota status.hard property.", zap.Error(err))
	} else if found {
		for k, v := range statusHard {
			val := parseQuantityValue(k, v.(string))
			mb.RecordK8sHierarchicalResourceQuotaHardLimitDataPoint(ts, val, k)
		}
	}

	statusUsed, found, err := unstructured.NestedMap(hrq.Object, "status", "used")
	if err != nil {
		logger.Warn("Failed to get HierarchicalResourceQuota status.used property.", zap.Error(err))
	} else if found {
		for k, v := range statusUsed {
			val := parseQuantityValue(k, v.(string))
			mb.RecordK8sHierarchicalResourceQuotaUsedDataPoint(ts, val, k)
		}
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
