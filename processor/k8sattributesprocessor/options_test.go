// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func TestWithAPIConfig(t *testing.T) {
	p := &kubernetesprocessor{}
	apiConfig := k8sconfig.APIConfig{AuthType: "test-auth-type"}
	err := withAPIConfig(apiConfig)(p)
	require.EqualError(t, err, "invalid authType for kubernetes: test-auth-type")

	apiConfig = k8sconfig.APIConfig{AuthType: "kubeConfig"}
	err = withAPIConfig(apiConfig)(p)
	require.NoError(t, err)
	assert.Equal(t, apiConfig, p.apiConfig)
}

func TestWithFilterNamespace(t *testing.T) {
	p := &kubernetesprocessor{}
	assert.NoError(t, withFilterNamespace("testns")(p))
	assert.Equal(t, "testns", p.filters.Namespace)
}

func TestWithFilterNode(t *testing.T) {
	p := &kubernetesprocessor{}
	assert.NoError(t, withFilterNode("testnode", "")(p))
	assert.Equal(t, "testnode", p.filters.Node)

	p = &kubernetesprocessor{}
	assert.NoError(t, withFilterNode("testnode", "NODE_NAME")(p))
	assert.Empty(t, p.filters.Node)

	t.Setenv("NODE_NAME", "nodefromenv")
	p = &kubernetesprocessor{}
	assert.NoError(t, withFilterNode("testnode", "NODE_NAME")(p))
	assert.Equal(t, "nodefromenv", p.filters.Node)
}

func TestWithPassthrough(t *testing.T) {
	p := &kubernetesprocessor{}
	assert.NoError(t, withPassthrough()(p))
	assert.True(t, p.passthroughMode)
}

func TestEnabledAttributes(t *testing.T) {
	// This list needs to be updated when the defaults in metadata.yaml are updated.
	expected := []string{
		string(conventions.K8SNamespaceNameKey),
		string(conventions.K8SPodNameKey),
		string(conventions.K8SPodUIDKey),
		metadataPodStartTime,
		string(conventions.K8SDeploymentNameKey),
		string(conventions.K8SNodeNameKey),
		string(conventions.ContainerImageNameKey),
		string(conventions.ContainerImageTagKey),
	}
	assert.ElementsMatch(t, expected, enabledAttributes())
}

func TestWithExtractAnnotations(t *testing.T) {
	tests := []struct {
		name      string
		args      []FieldExtractConfig
		want      []kube.FieldExtractionRule
		wantError string
	}{
		{
			"empty",
			[]FieldExtractConfig{},
			nil,
			"",
		},
		{
			"basic",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromPod,
				},
			},
			"",
		},
		{
			"basic-namespace",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromNamespace,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromNamespace,
				},
			},
			"",
		},
		{
			"basic-node",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromNode,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromNode,
				},
			},
			"",
		},
		{
			"basic-pod-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromPod,
				},
			},
			"",
		},
		{
			"basic-namespace-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromNamespace,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromNamespace,
				},
			},
			"",
		},
		{
			"basic-node-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromNode,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromNode,
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExtractAnnotations(tt.args...)
			err := opt(p)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, p.rules.Annotations)
			}
		})
	}
}

func TestWithExtractLabels(t *testing.T) {
	tests := []struct {
		name      string
		args      []FieldExtractConfig
		want      []kube.FieldExtractionRule
		wantError string
	}{
		{
			"empty",
			[]FieldExtractConfig{},
			nil,
			"",
		},
		{
			"basic",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromPod,
				},
			},
			"",
		},
		{
			"basic-namespace",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromNamespace,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromNamespace,
				},
			},
			"",
		},
		{
			"basic-node",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					From:    kube.MetadataFromNode,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
					From: kube.MetadataFromNode,
				},
			},
			"",
		},
		{
			"basic-pod-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromPod,
				},
			},
			"",
		},
		{
			"basic-namespace-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromNamespace,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromNamespace,
				},
			},
			"",
		},
		{
			"basic-node-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
					From:     kube.MetadataFromNode,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
					From:     kube.MetadataFromNode,
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExtractLabels(tt.args...)
			err := opt(p)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, p.rules.Labels)
			}
		})
	}
}

func TestWithExtractMetadata(t *testing.T) {
	p := &kubernetesprocessor{}
	assert.NoError(t, withExtractMetadata(enabledAttributes()...)(p))
	assert.True(t, p.rules.Namespace)
	assert.True(t, p.rules.PodName)
	assert.True(t, p.rules.PodUID)
	assert.True(t, p.rules.StartTime)
	assert.True(t, p.rules.DeploymentName)
	assert.True(t, p.rules.Node)

	p = &kubernetesprocessor{}
	assert.NoError(t, withExtractMetadata(string(conventions.K8SNamespaceNameKey), string(conventions.K8SPodNameKey), string(conventions.K8SPodUIDKey))(p))
	assert.True(t, p.rules.Namespace)
	assert.True(t, p.rules.PodName)
	assert.True(t, p.rules.PodUID)
	assert.False(t, p.rules.StartTime)
	assert.False(t, p.rules.DeploymentName)
	assert.False(t, p.rules.Node)
}

func TestWithFilterLabels(t *testing.T) {
	tests := []struct {
		name  string
		args  []FieldFilterConfig
		want  []kube.LabelFilter
		error string
	}{
		{
			"empty",
			[]FieldFilterConfig{},
			nil,
			"",
		},
		{
			"default",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
				},
			},
			[]kube.LabelFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
			},
			"",
		},
		{
			"equals",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "equals",
				},
			},
			[]kube.LabelFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
			},
			"",
		},
		{
			"not-equals",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "not-equals",
				},
			},
			[]kube.LabelFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.NotEquals,
				},
			},
			"",
		},
		{
			"exists",
			[]FieldFilterConfig{
				{
					Key: "k1",
					Op:  "exists",
				},
			},
			[]kube.LabelFilter{
				{
					Key: "k1",
					Op:  selection.Exists,
				},
			},
			"",
		},
		{
			"does-not-exist",
			[]FieldFilterConfig{
				{
					Key: "k1",
					Op:  "does-not-exist",
				},
			},
			[]kube.LabelFilter{
				{
					Key: "k1",
					Op:  selection.DoesNotExist,
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withFilterLabels(tt.args...)
			err := opt(p)
			if tt.error != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.error)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, p.filters.Labels)
		})
	}
}

func TestWithFilterFields(t *testing.T) {
	tests := []struct {
		name  string
		args  []FieldFilterConfig
		want  []kube.FieldFilter
		error string
	}{
		{
			"empty",
			[]FieldFilterConfig{},
			nil,
			"",
		},
		{
			"default",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
				},
			},
			[]kube.FieldFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
			},
			"",
		},
		{
			"equals",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "equals",
				},
			},
			[]kube.FieldFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
			},
			"",
		},
		{
			"not-equals",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "not-equals",
				},
			},
			[]kube.FieldFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.NotEquals,
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withFilterFields(tt.args...)
			err := opt(p)
			if tt.error != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.error)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, p.filters.Fields)
		})
	}
}

func Test_extractFieldRules(t *testing.T) {
	type args struct {
		fieldType string
		fields    []FieldExtractConfig
	}
	tests := []struct {
		name    string
		args    args
		want    []kube.FieldExtractionRule
		wantErr bool
	}{
		{
			name: "default",
			args: args{"labels", []FieldExtractConfig{
				{
					Key:  "key",
					From: kube.MetadataFromPod,
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name: "k8s.pod.labels.key",
					Key:  "key",
					From: kube.MetadataFromPod,
				},
			},
		},
		{
			name: "basic",
			args: args{"field", []FieldExtractConfig{
				{
					TagName: "name",
					Key:     "key",
					From:    kube.MetadataFromPod,
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name: "name",
					Key:  "key",
					From: kube.MetadataFromPod,
				},
			},
		},
		{
			name: "keyregex-capture-group",
			args: args{"labels", []FieldExtractConfig{
				{
					TagName:  "$0-$1-$2",
					KeyRegex: "(key)(.*)",
					From:     kube.MetadataFromPod,
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name:                 "$0-$1-$2",
					KeyRegex:             regexp.MustCompile("^(?:(key)(.*))$"),
					HasKeyRegexReference: true,
					From:                 kube.MetadataFromPod,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFieldRules(tt.args.fieldType, tt.args.fields...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWithExtractPodAssociation(t *testing.T) {
	tests := []struct {
		name string
		args []PodAssociationConfig
		want []kube.Association
	}{
		{
			"empty",
			[]PodAssociationConfig{},
			[]kube.Association{},
		},
		{
			"basic",
			[]PodAssociationConfig{
				{
					Sources: []PodAssociationSourceConfig{
						{
							From: "label",
							Name: "ip",
						},
					},
				},
			},
			[]kube.Association{
				{
					Sources: []kube.AssociationSource{
						{
							From: "label",
							Name: "ip",
						},
					},
				},
			},
		},
		{
			"connection",
			[]PodAssociationConfig{
				{
					Sources: []PodAssociationSourceConfig{
						{
							From: "connection",
							Name: "ip",
						},
					},
				},
			},
			[]kube.Association{
				{
					Sources: []kube.AssociationSource{
						{
							From: "connection",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExtractPodAssociations(tt.args...)
			assert.NoError(t, opt(p))
			assert.Equal(t, tt.want, p.podAssociations)
		})
	}
}

func TestWithExcludes(t *testing.T) {
	tests := []struct {
		name string
		args ExcludeConfig
		want kube.Excludes
	}{
		{
			"default",
			ExcludeConfig{},
			kube.Excludes{
				Pods: []kube.ExcludePods{
					{Name: regexp.MustCompile(`jaeger-agent`)},
					{Name: regexp.MustCompile(`jaeger-collector`)},
				},
			},
		},
		{
			"configured",
			ExcludeConfig{
				Pods: []ExcludePodConfig{
					{Name: "ignore_pod1"},
					{Name: "ignore_pod2"},
				},
			},
			kube.Excludes{
				Pods: []kube.ExcludePods{
					{Name: regexp.MustCompile(`ignore_pod1`)},
					{Name: regexp.MustCompile(`ignore_pod2`)},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExcludes(tt.args)
			assert.NoError(t, opt(p))
			assert.Equal(t, tt.want, p.podIgnore)
		})
	}
}

func TestOtelAnnotations(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		wantAnnotations []kube.FieldExtractionRule
	}{
		{
			name: "no otel annotations",
		},
		{
			name:    "with otel annotations",
			enabled: true,
			wantAnnotations: []kube.FieldExtractionRule{
				{
					Name:                 "$1",
					KeyRegex:             regexp.MustCompile(`^resource\.opentelemetry\.io/(.+)$`),
					HasKeyRegexReference: true,
					From:                 kube.MetadataFromPod,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := kubernetesprocessor{}
			rules := withOtelAnnotations(tt.enabled)
			err := rules(&p)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAnnotations, p.rules.Annotations)
		})
	}
}
