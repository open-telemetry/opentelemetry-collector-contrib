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

package container

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractFieldRules(t *testing.T) {
	type args struct {
		fieldType string
		fields    []FieldExtractConfig
	}
	var tests = []struct {
		name     string
		args     args
		expected []FieldExtractionRule
		wantErr  bool
	}{
		{
			name: "default",
			args: args{
				fieldType: "labels",
				fields: []FieldExtractConfig{
					{
						Key: "key",
					},
				},
			},
			expected: []FieldExtractionRule{
				{
					Name: "container.labels.key",
					Key:  "key",
				},
			},
			wantErr: false,
		},
		{
			name: "basic",
			args: args{
				fieldType: "field",
				fields: []FieldExtractConfig{
					{
						TagName: "name",
						Key:     "key",
					},
				},
			},
			expected: []FieldExtractionRule{
				{
					Name: "name",
					Key:  "key",
				},
			},
		},
		{
			name: "regex-without-match",
			args: args{
				fieldType: "field",
				fields: []FieldExtractConfig{
					{
						TagName: "name",
						Key:     "key",
						Regex:   "^h$",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "badregex",
			args: args{
				fieldType: "field",
				fields: []FieldExtractConfig{
					{
						TagName: "name",
						Key:     "key",
						Regex:   "[",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "keyregex-capture-group",
			args: args{
				fieldType: "labels",
				fields: []FieldExtractConfig{
					{
						TagName:  "$0-$1-$2",
						KeyRegex: "(key)(.*)",
					},
				},
			},
			expected: []FieldExtractionRule{
				{
					Name:                 "$0-$1-$2",
					KeyRegex:             regexp.MustCompile("^(?:(key)(.*))$"),
					HasKeyRegexReference: true,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			actual, err := ExtractFieldRules(tt.args.fieldType, tt.args.fields)
			if tt.wantErr {
				assert.Error(t, err)
			} else if assert.NoError(t, err) {
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}
