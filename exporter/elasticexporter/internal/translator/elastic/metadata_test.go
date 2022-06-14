// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elastic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.elastic.co/apm/model"
	"go.elastic.co/apm/transport/transporttest"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestMetadataDefaults(t *testing.T) {
	out := metadataWithResource(t, pcommon.NewResource())
	assert.Equal(t, metadata{
		service: model.Service{
			Name: "unknown",
			Agent: &model.Agent{
				Name:    "otlp",
				Version: "unknown",
			},
		},
	}, out)
}

func TestMetadataServiceName(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"service.name": "foo",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.service.Name)
}

func TestMetadataServiceVersion(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"service.version": "1.2.3",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "1.2.3", out.service.Version)
}

func TestMetadataServiceInstance(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"service.instance.id": "foo-1",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, &model.ServiceNode{
		ConfiguredName: "foo-1",
	}, out.service.Node)
}

func TestMetadataServiceEnvironment(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"deployment.environment": "foo",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.service.Environment)
}

func TestMetadataSystemHostname(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"host.name": "foo",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.system.Hostname)
}

func TestMetadataServiceLanguageName(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"telemetry.sdk.language": "java",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "java", out.service.Language.Name)
	assert.Equal(t, "otlp/java", out.service.Agent.Name)
}

func TestMetadataAgentName(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"telemetry.sdk.name":    "foo",
		"telemetry.sdk.version": "bar",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, &model.Agent{
		Name:    "foo",
		Version: "bar",
	}, out.service.Agent)
}

func TestMetadataLabels(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"string": "abc",
		"int":    123,
		"double": 123.456,
		"bool":   true,
		// well known resource label, not carried across
		conventions.AttributeServiceVersion: "...",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, model.IfaceMap{
		{Key: "bool", Value: true},
		{Key: "double", Value: 123.456},
		{Key: "int", Value: 123.0},
		{Key: "string", Value: "abc"},
	}, out.labels)
}

func TestMetadataKubernetes(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]interface{}{
		"k8s.namespace.name": "namespace_name",
		"k8s.pod.name":       "pod_name",
		"k8s.pod.uid":        "pod_uid",
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, &model.Kubernetes{
		Namespace: "namespace_name",
		Pod: &model.KubernetesPod{
			Name: "pod_name",
			UID:  "pod_uid",
		},
	}, out.system.Kubernetes)
}

func resourceFromAttributesMap(attrs map[string]interface{}) pcommon.Resource {
	resource := pcommon.NewResource()
	pcommon.NewMapFromRaw(attrs).CopyTo(resource.Attributes())
	return resource
}

func metadataWithResource(t *testing.T, resource pcommon.Resource) metadata {
	var out metadata
	var recorder transporttest.RecorderTransport
	var w fastjson.Writer
	assert.NoError(t, elastic.EncodeResourceMetadata(resource, &w))
	sendStream(t, &w, &recorder)
	out.system, out.process, out.service, out.labels = recorder.Metadata()
	return out
}

type metadata struct {
	system  model.System
	process model.Process
	service model.Service
	labels  model.IfaceMap
}
