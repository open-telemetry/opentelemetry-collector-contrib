// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

func watchObjectsToLogData(event *watch.Event, config *K8sObjectsConfig) plog.Logs {
	udata := event.Object.(*unstructured.Unstructured)
	ul := unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{{
			Object: map[string]interface{}{
				"type":   string(event.Type),
				"object": udata.Object,
			},
		}},
	}
	eventName := fmt.Sprintf("Watch %s", config.gvr.Resource)
	return unstructuredListToLogData(&ul, config, eventName)
}

func pullObjectsToLogData(event *unstructured.UnstructuredList, config *K8sObjectsConfig) plog.Logs {
	eventName := fmt.Sprintf("Pull %s", config.gvr.Resource)
	return unstructuredListToLogData(event, config, eventName)
}
func unstructuredListToLogData(event *unstructured.UnstructuredList, config *K8sObjectsConfig, eventName string) plog.Logs {
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

		attrs := record.Attributes()
		attrs.EnsureCapacity(2)
		attrs.PutStr("event.domain", "k8s")
		attrs.PutStr("event.name", eventName)
		attrs.PutStr("k8s.resource.name", config.gvr.Resource)

		dest := record.Body()
		destMap := dest.SetEmptyMap()
		//nolint:errcheck
		destMap.FromRaw(e.Object)
	}
	return out
}
