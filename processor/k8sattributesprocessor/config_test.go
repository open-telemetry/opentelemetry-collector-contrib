// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id            component.ID
		expected      component.Config
		disallowRegex bool
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Exclude:   ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				Passthrough: false,
				Extract: ExtractConfig{
					Metadata: []string{"k8s.pod.name", "k8s.pod.uid", "k8s.pod.ip", "k8s.deployment.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.start_time", "k8s.cluster.uid"},
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
						Sources: []PodAssociationSourceConfig{
							{
								From: "resource_attribute",
								Name: "ip",
							},
						},
					},
					{
						Sources: []PodAssociationSourceConfig{
							{
								From: "resource_attribute",
								Name: "k8s.pod.ip",
							},
						},
					},
					{
						Sources: []PodAssociationSourceConfig{
							{
								From: "resource_attribute",
								Name: "host.name",
							},
						},
					},
					{
						Sources: []PodAssociationSourceConfig{
							{
								From: "connection",
								Name: "ip",
							},
						},
					},
				},
				Exclude: ExcludeConfig{
					Pods: []ExcludePodConfig{
						{Name: "jaeger-agent"},
						{Name: "jaeger-collector"},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				Passthrough: false,
				Extract: ExtractConfig{
					Annotations: []FieldExtractConfig{
						{KeyRegex: "opentel.*", From: kube.MetadataFromPod},
					},
					Labels: []FieldExtractConfig{
						{KeyRegex: "opentel.*", From: kube.MetadataFromPod},
					},
					Metadata: enabledAttributes(),
				},
				Exclude: ExcludeConfig{
					Pods: []ExcludePodConfig{
						{Name: "jaeger-agent"},
						{Name: "jaeger-collector"},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "deprecated-regex"),
			expected: &Config{
				APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				Passthrough: false,
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Annotations: []FieldExtractConfig{
						{Regex: "field=(?P<value>.+)", From: "pod"},
					},
					Labels: []FieldExtractConfig{
						{Regex: "field=(?P<value>.+)", From: "pod"},
					},
				},
				Exclude: ExcludeConfig{
					Pods: []ExcludePodConfig{
						{Name: "jaeger-agent"},
						{Name: "jaeger-collector"},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "too_many_sources"),
		},
		{
			id:            component.NewIDWithName(metadata.Type, "deprecated-regex"),
			disallowRegex: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_keys_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_keys_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_from_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_from_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_keyregex_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_keyregex_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_groups_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_groups_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_name_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_regex_name_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_filter_label_op"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_filter_field_op"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			if tt.disallowRegex {
				require.NoError(t, featuregate.GlobalRegistry().Set(disallowFieldExtractConfigRegex.ID(), true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(disallowFieldExtractConfigRegex.ID(), false))
				})
			}
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				err = component.ValidateConfig(cfg)
				assert.Error(t, err)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
