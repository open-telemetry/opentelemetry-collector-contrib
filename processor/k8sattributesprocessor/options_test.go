// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func TestWithAPIConfig(t *testing.T) {
	p := &kubernetesprocessor{}
	apiConfig := k8sconfig.APIConfig{AuthType: "test-auth-type"}
	err := withAPIConfig(apiConfig)(p)
	assert.Error(t, err)
	assert.Equal(t, "invalid authType for kubernetes: test-auth-type", err.Error())

	apiConfig = k8sconfig.APIConfig{AuthType: "kubeConfig"}
	err = withAPIConfig(apiConfig)(p)
	assert.NoError(t, err)
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
	assert.Equal(t, "", p.filters.Node)

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
		conventions.AttributeK8SNamespaceName,
		conventions.AttributeK8SPodName,
		conventions.AttributeK8SPodUID,
		metadataPodStartTime,
		conventions.AttributeK8SDeploymentName,
		conventions.AttributeK8SNodeName,
		conventions.AttributeContainerImageName,
		conventions.AttributeContainerImageTag,
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
			"bad",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					Regex:   "[",
					From:    kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{},
			"error parsing regexp: missing closing ]: `[`",
		},
		{
			"basic",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					Regex:   "field=(?P<value>.+)",
					From:    kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:  "tag1",
					Key:   "key1",
					Regex: regexp.MustCompile(`field=(?P<value>.+)`),
					From:  kube.MetadataFromPod,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExtractAnnotations(tt.args...)
			err := opt(p)
			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.wantError, err.Error())
				return
			}
			got := p.rules.Annotations
			assert.Equal(t, tt.want, got)
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
			"bad",
			[]FieldExtractConfig{{
				TagName: "t1",
				Key:     "k1",
				Regex:   "[",
				From:    kube.MetadataFromPod,
			}},
			[]kube.FieldExtractionRule{},
			"error parsing regexp: missing closing ]: `[`",
		},
		{
			"basic",
			[]FieldExtractConfig{
				{
					TagName: "tag1",
					Key:     "key1",
					Regex:   "field=(?P<value>.+)",
					From:    kube.MetadataFromPod,
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:  "tag1",
					Key:   "key1",
					Regex: regexp.MustCompile(`field=(?P<value>.+)`),
					From:  kube.MetadataFromPod,
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
			"basic-namespace",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &kubernetesprocessor{}
			opt := withExtractLabels(tt.args...)
			err := opt(p)
			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.wantError, err.Error())
				return
			}
			assert.Equal(t, tt.want, p.rules.Labels)
		})
	}
}

func TestWithExtractMetadata(t *testing.T) {
	p := &kubernetesprocessor{}
	assert.NoError(t, withExtractMetadata()(p))
	assert.True(t, p.rules.Namespace)
	assert.True(t, p.rules.PodName)
	assert.True(t, p.rules.PodUID)
	assert.True(t, p.rules.StartTime)
	assert.True(t, p.rules.DeploymentName)
	assert.True(t, p.rules.Node)

	p = &kubernetesprocessor{}
	err := withExtractMetadata("randomfield")(p)
	assert.Error(t, err)
	assert.Equal(t, `"randomfield" is not a supported metadata field`, err.Error())

	p = &kubernetesprocessor{}
	assert.NoError(t, withExtractMetadata(conventions.AttributeK8SNamespaceName, conventions.AttributeK8SPodName, conventions.AttributeK8SPodUID)(p))
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
		{
			"exists",
			[]FieldFilterConfig{
				{
					Key: "k1",
					Op:  "exists",
				},
			},
			[]kube.FieldFilter{
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
			[]kube.FieldFilter{
				{
					Key: "k1",
					Op:  selection.DoesNotExist,
				},
			},
			"",
		},
		{
			"unknown",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "unknown-op",
				},
			},
			[]kube.FieldFilter{},
			"'unknown-op' is not a valid label filter operation for key=k1, value=v1",
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
		{
			"exists",
			[]FieldFilterConfig{
				{
					Key: "k1",
					Op:  "exists",
				},
			},
			[]kube.FieldFilter{
				{
					Key: "k1",
					Op:  selection.Exists,
				},
			},
			"'exists' is not a valid field filter operation for key=k1, value=",
		},
		{
			"does-not-exist",
			[]FieldFilterConfig{
				{
					Key: "k1",
					Op:  "does-not-exist",
				},
			},
			[]kube.FieldFilter{
				{
					Key: "k1",
					Op:  selection.DoesNotExist,
				},
			},
			"'does-not-exist' is not a valid field filter operation for key=k1, value=",
		},
		{
			"unknown",
			[]FieldFilterConfig{
				{
					Key:   "k1",
					Value: "v1",
					Op:    "unknown-op",
				},
			},
			[]kube.FieldFilter{},
			"'unknown-op' is not a valid field filter operation for key=k1, value=v1",
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
			name: "regex-without-match",
			args: args{"field", []FieldExtractConfig{
				{
					TagName: "name",
					Key:     "key",
					Regex:   "^h$",
					From:    kube.MetadataFromPod,
				},
			}},
			wantErr: true,
		},
		{
			name: "badregex",
			args: args{"field", []FieldExtractConfig{
				{
					TagName: "name",
					Key:     "key",
					Regex:   "[",
					From:    kube.MetadataFromPod,
				},
			}},
			wantErr: true,
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
