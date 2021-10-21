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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestMetadataDefaults(t *testing.T) {
	out := metadataWithResource(t, pdata.NewResource())
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
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"service.name": pdata.NewAttributeValueString("foo"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.service.Name)
}

func TestMetadataServiceVersion(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"service.version": pdata.NewAttributeValueString("1.2.3"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "1.2.3", out.service.Version)
}

func TestMetadataServiceInstance(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"service.instance.id": pdata.NewAttributeValueString("foo-1"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, &model.ServiceNode{
		ConfiguredName: "foo-1",
	}, out.service.Node)
}

func TestMetadataServiceEnvironment(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"deployment.environment": pdata.NewAttributeValueString("foo"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.service.Environment)
}

func TestMetadataSystemHostname(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"host.name": pdata.NewAttributeValueString("foo"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "foo", out.system.Hostname)
}

func TestMetadataServiceLanguageName(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"telemetry.sdk.language": pdata.NewAttributeValueString("java"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, "java", out.service.Language.Name)
	assert.Equal(t, "otlp/java", out.service.Agent.Name)
}

func TestMetadataAgentName(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"telemetry.sdk.name":    pdata.NewAttributeValueString("foo"),
		"telemetry.sdk.version": pdata.NewAttributeValueString("bar"),
	})
	out := metadataWithResource(t, resource)
	assert.Equal(t, &model.Agent{
		Name:    "foo",
		Version: "bar",
	}, out.service.Agent)
}

func TestMetadataLabels(t *testing.T) {
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"string": pdata.NewAttributeValueString("abc"),
		"int":    pdata.NewAttributeValueInt(123),
		"double": pdata.NewAttributeValueDouble(123.456),
		"bool":   pdata.NewAttributeValueBool(true),
		// well known resource label, not carried across
		conventions.AttributeServiceVersion: pdata.NewAttributeValueString("..."),
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
	resource := resourceFromAttributesMap(map[string]pdata.AttributeValue{
		"k8s.namespace.name": pdata.NewAttributeValueString("namespace_name"),
		"k8s.pod.name":       pdata.NewAttributeValueString("pod_name"),
		"k8s.pod.uid":        pdata.NewAttributeValueString("pod_uid"),
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

func resourceFromAttributesMap(attrs map[string]pdata.AttributeValue) pdata.Resource {
	resource := pdata.NewResource()
	resource.Attributes().InitFromMap(attrs)
	return resource
}

func metadataWithResource(t *testing.T, resource pdata.Resource) metadata {
	var out metadata
	var recorder transporttest.RecorderTransport
	var w fastjson.Writer
	elastic.EncodeResourceMetadata(resource, &w)
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
