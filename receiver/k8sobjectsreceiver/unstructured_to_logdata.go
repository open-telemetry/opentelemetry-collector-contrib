// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

type attrUpdaterFunc func(pcommon.Map)

func watchObjectsToLogData(event *watch.Event, observedAt time.Time, config *K8sObjectsConfig) plog.Logs {
	udata := event.Object.(*unstructured.Unstructured)
	ul := unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{{
			Object: map[string]interface{}{
				"type":   string(event.Type),
				"object": udata.Object,
			},
		}},
	}

	return unstructuredListToLogData(&ul, observedAt, config, func(attrs pcommon.Map) {
		objectMeta := udata.Object["metadata"].(map[string]interface{})
		name := objectMeta["name"].(string)
		if name != "" {
			attrs.PutStr("event.domain", "k8s")
			attrs.PutStr("event.name", name)
		}
	})
}

func pullObjectsToLogData(event *unstructured.UnstructuredList, observedAt time.Time, config *K8sObjectsConfig) plog.Logs {
	return unstructuredListToLogData(event, observedAt, config)
}

func unstructuredListToLogData(event *unstructured.UnstructuredList, observedAt time.Time, config *K8sObjectsConfig, attrUpdaters ...attrUpdaterFunc) plog.Logs {
	out := plog.NewLogs()
	resourceLogs := out.ResourceLogs()
	namespaceResourceMap := make(map[string]plog.LogRecordSlice)

	for _, e := range event.Items {
		logSlice, ok := namespaceResourceMap[e.GetNamespace()]
		if !ok {
			rl := resourceLogs.AppendEmpty()
			resourceAttrs := rl.Resource().Attributes()
			if namespace := e.GetNamespace(); namespace != "" {
				resourceAttrs.PutStr(semconv.AttributeK8SNamespaceName, namespace)
			}
			sl := rl.ScopeLogs().AppendEmpty()
			logSlice = sl.LogRecords()
			namespaceResourceMap[e.GetNamespace()] = logSlice
		}
		record := logSlice.AppendEmpty()
		record.SetObservedTimestamp(pcommon.NewTimestampFromTime(observedAt))

		attrs := record.Attributes()
		attrs.PutStr("k8s.resource.name", config.gvr.Resource)

		for _, attrUpdate := range attrUpdaters {
			attrUpdate(attrs)
		}

		dest := record.Body()
		destMap := dest.SetEmptyMap()
		//nolint:errcheck
		destMap.FromRaw(e.Object)
	}
	return out
}
