// Copyright 2019 OpenTelemetry Authors
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

package sourceprocessor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfgPath := path.Join(".", "testdata", "config.yaml")
	cfg, err := configtest.LoadConfigFile(t, cfgPath, factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	p1 := cfg.Processors["source"]
	assert.Equal(t, p1, factory.CreateDefaultConfig())

	p2 := cfg.Processors["source/2"]
	assert.Equal(t, p2, &Config{
		ProcessorSettings: &config.ProcessorSettings{
			TypeVal: "source",
			NameVal: "source/2",
		},

		Collector:                 "somecollector",
		Source:                    "tracesource",
		SourceName:                "%{namespace}.%{pod}.%{container}/foo",
		SourceCategory:            "%{namespace}/%{pod_name}/bar",
		SourceCategoryPrefix:      "kubernetes/",
		SourceCategoryReplaceDash: "/",
		ExcludeContainerRegex:     "excluded_container_regex",
		ExcludeHostRegex:          "excluded_host_regex",
		ExcludeNamespaceRegex:     "excluded_namespace_regex",
		ExcludePodRegex:           "excluded_pod_regex",

		AnnotationPrefix:   "pod_annotation_",
		ContainerKey:       "container",
		NamespaceKey:       "namespace",
		PodKey:             "pod",
		PodIDKey:           "pod_id",
		PodNameKey:         "pod_name",
		PodTemplateHashKey: "pod_labels_pod-template-hash",
		SourceHostKey:      "source_host",
	})
}
