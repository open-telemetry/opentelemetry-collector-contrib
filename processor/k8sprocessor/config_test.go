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
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Processors[configmodels.Type(typeStr)] = factory
	require.NoError(t, err)

	err = configcheck.ValidateConfig(factory.CreateDefaultConfig())
	require.NoError(t, err)

	config, err := configtest.LoadConfigFile(
		t,
		path.Join(".", "testdata", "config.yaml"),
		factories)

	require.Nil(t, err)
	require.NotNil(t, config)

	p0 := config.Processors["k8s_tagger"]
	assert.Equal(t, p0,
		&Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: "k8s_tagger",
				NameVal: "k8s_tagger",
			},
			APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		})

	p1 := config.Processors["k8s_tagger/2"]
	assert.Equal(t, p1,
		&Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: "k8s_tagger",
				NameVal: "k8s_tagger/2",
			},
			APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
			Passthrough:        false,
			OwnerLookupEnabled: true,
			Extract: ExtractConfig{
				Metadata: []string{
					"containerId", "containerName", "containerImage", "clusterName", "daemonSetName",
					"deploymentName", "hostName", "namespace", "nodeName", "podId", "podName",
					"replicaSetName", "serviceName", "startTime", "statefulSetName",
				},
				Tags: map[string]string{
					"containerid": "my.namespace.containerId",
				},
				Annotations: []FieldExtractConfig{
					{TagName: "a1", Key: "annotation-one"},
					{TagName: "a2", Key: "annotation-two", Regex: "field=(?P<value>.+)"},
				},
				Labels: []FieldExtractConfig{
					{TagName: "l1", Key: "label1"},
					{TagName: "l2", Key: "label2", Regex: "field=(?P<value>.+)"},
				},
				NamespaceLabels: []FieldExtractConfig{
					{TagName: "namespace_labels_%s", Key: "*"},
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
		})
}
