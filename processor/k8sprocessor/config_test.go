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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	require.NoError(t, err)
	factory := &Factory{}
	factories.Processors[configmodels.Type(typeStr)] = factory
	require.NoError(t, err)

	err = configcheck.ValidateConfig(factory.CreateDefaultConfig())
	require.NoError(t, err)

	config, err := config.LoadConfigFile(
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
			APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
			Passthrough: false,
			Extract: ExtractConfig{
				Metadata: []string{"podName", "podUID", "deployment", "cluster", "namespace", "node", "startTime"},
				Annotations: []FieldExtractConfig{
					{TagName: "a1", Key: "annotation-one"},
					{TagName: "a2", Key: "annotation-two", Regex: "field=(?P<value>.+)"},
				},
				Labels: []FieldExtractConfig{
					{TagName: "l1", Key: "label1"},
					{TagName: "l2", Key: "label2", Regex: "field=(?P<value>.+)"},
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
