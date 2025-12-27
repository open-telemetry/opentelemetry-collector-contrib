// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Exclude:   ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
				WaitForMetadataTimeout: 10 * time.Second,
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
						{TagName: "a2", Key: "annotation-two", From: kube.MetadataFromPod},
					},
					Labels: []FieldExtractConfig{
						{TagName: "l1", Key: "label1", From: "pod"},
						{TagName: "l2", Key: "label2", From: kube.MetadataFromPod},
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
				WaitForMetadataTimeout: 10 * time.Second,
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
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "deployment_name_from_replicaset"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata:                     enabledAttributes(),
					DeploymentNameFromReplicaSet: true,
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "too_many_sources"),
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
			id: component.NewIDWithName(metadata.Type, "bad_keyregex_labels"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_keyregex_annotations"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_filter_label_op"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_filter_field_op"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "otel_annotations"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata:        enabledAttributes(),
					OtelAnnotations: true,
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "wait_for_metadata"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
				Exclude:                defaultExcludes,
				WaitForMetadata:        true,
				WaitForMetadataTimeout: 30 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "passthrough_mode"),
			expected: &Config{
				APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Passthrough: true,
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "filter_label_exists"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
				Filter: FilterConfig{
					Labels: []FieldFilterConfig{
						{Key: "app", Op: "exists"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "filter_label_does_not_exist"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
				},
				Filter: FilterConfig{
					Labels: []FieldFilterConfig{
						{Key: "deprecated-label", Op: "does-not-exist"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_namespace"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Labels: []FieldExtractConfig{
						{TagName: "ns_label", Key: "team", From: "namespace"},
					},
					Annotations: []FieldExtractConfig{
						{TagName: "ns_annotation", Key: "owner", From: "namespace"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_node"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: []string{"k8s.node.name", "k8s.node.uid"},
					Labels: []FieldExtractConfig{
						{TagName: "node_label", Key: "node-role", From: "node"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_deployment"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Labels: []FieldExtractConfig{
						{TagName: "deployment_label", Key: "app", From: "deployment"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_statefulset"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Labels: []FieldExtractConfig{
						{TagName: "statefulset_label", Key: "app", From: "statefulset"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_daemonset"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Labels: []FieldExtractConfig{
						{TagName: "daemonset_label", Key: "app", From: "daemonset"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "extract_from_job"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: enabledAttributes(),
					Labels: []FieldExtractConfig{
						{TagName: "job_label", Key: "app", From: "job"},
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_metadata_fields"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				Extract: ExtractConfig{
					Metadata: []string{
						"k8s.namespace.name", "k8s.pod.name", "k8s.pod.uid", "k8s.pod.hostname",
						"k8s.pod.start_time", "k8s.pod.ip", "k8s.deployment.name", "k8s.deployment.uid",
						"k8s.replicaset.name", "k8s.replicaset.uid", "k8s.daemonset.name", "k8s.daemonset.uid",
						"k8s.statefulset.name", "k8s.statefulset.uid", "k8s.job.name", "k8s.job.uid",
						"k8s.cronjob.name", "k8s.cronjob.uid", "k8s.node.name", "k8s.node.uid",
						"k8s.container.name", "container.id", "container.image.name", "container.image.tag",
						"container.image.repo_digests", "service.namespace", "service.name",
						"service.version", "service.instance.id", "k8s.cluster.uid",
					},
				},
				Exclude:                defaultExcludes,
				WaitForMetadataTimeout: 10 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_metadata_field"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			// Set "K8S_NODE" to pass validation.
			t.Setenv("K8S_NODE", "ip-111.us-west-2.compute.internal")
			if tt.expected == nil {
				err = xconfmap.Validate(cfg)
				assert.Error(t, err)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestFilterConfigInvalidEnvVar(t *testing.T) {
	f := FilterConfig{
		Namespace:      "ns2",
		NodeFromEnvVar: "K8S_NODE",
		Labels:         []FieldFilterConfig{},
		Fields:         []FieldFilterConfig{},
	}
	assert.Error(t, xconfmap.Validate(f))
}
