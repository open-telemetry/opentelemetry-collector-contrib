// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/kube"
)

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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
				},
			},
			"",
		},
		{
			"basic-event-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &k8seventsReceiver{}
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name: "tag1",
					Key:  "key1",
				},
			},
			"",
		},
		{
			"basic-event-keyregex",
			[]FieldExtractConfig{
				{
					TagName:  "tag1",
					KeyRegex: "key*",
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
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
				},
			},
			[]kube.FieldExtractionRule{
				{
					Name:     "tag1",
					KeyRegex: regexp.MustCompile("^(?:key*)$"),
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &k8seventsReceiver{}
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
					Key: "key",
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name: "k8s.event.labels.key",
					Key:  "key",
				},
			},
		},
		{
			name: "basic",
			args: args{"field", []FieldExtractConfig{
				{
					TagName: "name",
					Key:     "key",
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name: "name",
					Key:  "key",
				},
			},
		},
		{
			name: "keyregex-capture-group",
			args: args{"labels", []FieldExtractConfig{
				{
					TagName:  "$0-$1-$2",
					KeyRegex: "(key)(.*)",
				},
			}},
			want: []kube.FieldExtractionRule{
				{
					Name:                 "$0-$1-$2",
					KeyRegex:             regexp.MustCompile("^(?:(key)(.*))$"),
					HasKeyRegexReference: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractFieldRules(tt.args.fieldType, tt.args.fields...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
