// Copyright 2020 OpenTelemetry Authors
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

package k8sprocessor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Processors[typeStr] = factory
	require.NoError(t, err)

	err = configcheck.ValidateConfig(factory.CreateDefaultConfig())
	require.NoError(t, err)

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewID(typeStr)]
	assert.Equal(t, p0,
		&Config{
			ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
			APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
			Exclude:           ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}},
		})

	p1 := cfg.Processors[config.NewIDWithName(typeStr, "2")]
	assert.Equal(t, p1,
		&Config{
			ProcessorSettings: config.NewProcessorSettings(config.NewIDWithName(typeStr, "2")),
			APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
			Passthrough:       false,
			Extract: ExtractConfig{
				Metadata: []string{"k8s.pod.name", "k8s.pod.uid", "k8s.deployment.name", "k8s.cluster.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.start_time"},
				Annotations: []FieldExtractConfig{
					{TagName: "a1", Key: "annotation-one", From: "pod"},
					{TagName: "a2", Key: "annotation-two", Regex: "field=(?P<value>.+)", From: kube.MetadataFromPod},
				},
				Labels: []FieldExtractConfig{
					{TagName: "l1", Key: "label1", From: "pod"},
					{TagName: "l2", Key: "label2", Regex: "field=(?P<value>.+)", From: kube.MetadataFromPod},
				},
			},
			Filter: FilterConfig{
				Namespace:      "ns2",
				Node:           "ip-111.us-west-2.compute.internal",
				NodeFromEnvVar: "K8S_NODE",
				Labels: []FieldFilterConfig{
					{Key: "key1", Value: "value1"},
					{Key: "key2", Value: "value2", Op: "not-equals"},
				},
				Fields: []FieldFilterConfig{
					{Key: "key1", Value: "value1"},
					{Key: "key2", Value: "value2", Op: "not-equals"},
				},
			},
			Association: []PodAssociationConfig{
				{
					From: "resource_attribute",
					Name: "ip",
				},
				{
					From: "resource_attribute",
					Name: "k8s.pod.ip",
				},
				{
					From: "resource_attribute",
					Name: "host.name",
				},
				{
					From: "connection",
					Name: "ip",
				},
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
			Exclude: ExcludeConfig{
				Pods: []ExcludePodConfig{
					{Name: "jaeger-agent"},
					{Name: "jaeger-collector"},
				},
			},
		})
}
