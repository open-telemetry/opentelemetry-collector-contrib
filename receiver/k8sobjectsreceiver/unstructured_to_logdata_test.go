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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUnstructuredListToLogData(t *testing.T) {
	t.Parallel()

	t.Run("Test namespaced objects", func(t *testing.T) {
		objects := unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{},
		}
		namespaces := []string{"ns1", "ns1", "ns2", "ns2"}
		for i, namespace := range namespaces {
			object := unstructured.Unstructured{}
			object.SetKind("Pod")
			object.SetNamespace(namespace)
			object.SetName(fmt.Sprintf("pod-%d", i))
			objects.Items = append(objects.Items, object)
		}
		gvr := schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		}
		logs := unstructuredListToLogData(&objects, gvr)

		assert.Equal(t, logs.LogRecordCount(), 4)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 2)

		namespaces = []string{"ns1", "ns2"}
		for i, namespace := range namespaces {
			rl := resourceLogs.At(i)
			resourceAttributes := rl.Resource().Attributes()
			logRecords := rl.ScopeLogs().At(0).LogRecords()
			ns, _ := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
			assert.Equal(t, ns.AsString(), namespace)
			assert.Equal(t, rl.ScopeLogs().Len(), 1)
			assert.Equal(t, rl.ScopeLogs().At(0).LogRecords().Len(), 2)
			// valiadte event.domain and event.name attribute
			for j := 0; j < logRecords.Len(); j++ {
				domain, ok := logRecords.At(j).Attributes().Get("event.domain")
				require.Equal(t, true, ok)
				assert.Equal(t, "k8s", domain.AsString())

				name, ok := logRecords.At(j).Attributes().Get("event.name")
				require.Equal(t, true, ok)
				assert.Equal(t, "pods", name.AsString())
			}

		}
	})

	t.Run("Test non-namespaced objects", func(t *testing.T) {
		objects := unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{},
		}
		for i := 0; i < 3; i++ {
			object := unstructured.Unstructured{}
			object.SetKind("Node")
			object.SetName(fmt.Sprintf("node-%d", i))
			objects.Items = append(objects.Items, object)
		}

		gvr := schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "nodes",
		}

		logs := unstructuredListToLogData(&objects, gvr)

		assert.Equal(t, logs.LogRecordCount(), 3)

		resourceLogs := logs.ResourceLogs()
		assert.Equal(t, resourceLogs.Len(), 1)
		rl := resourceLogs.At(0)
		resourceAttributes := rl.Resource().Attributes()
		logRecords := rl.ScopeLogs().At(0).LogRecords()
		_, ok := resourceAttributes.Get(semconv.AttributeK8SNamespaceName)
		assert.Equal(t, ok, false)
		assert.Equal(t, rl.ScopeLogs().Len(), 1)
		assert.Equal(t, logRecords.Len(), 3)

		// valiadte event.domain and event.name attribute
		for i := 0; i < logRecords.Len(); i++ {
			domain, ok := logRecords.At(i).Attributes().Get("event.domain")
			require.Equal(t, true, ok)
			assert.Equal(t, "k8s", domain.AsString())

			name, ok := logRecords.At(i).Attributes().Get("event.name")
			require.Equal(t, true, ok)
			assert.Equal(t, "nodes", name.AsString())
		}

	})

}
