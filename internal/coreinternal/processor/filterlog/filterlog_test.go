// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

func createConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}

func TestLogRecord_validateMatchesConfiguration_InvalidConfig(t *testing.T) {
	testcases := []struct {
		name        string
		property    filterconfig.MatchProperties
		errorString string
	}{
		{
			name:        "empty_property",
			property:    filterconfig.MatchProperties{},
			errorString: `at least one of "attributes", "libraries", "resources" or "log_bodies" field must be specified`,
		},
		{
			name: "empty_log_bodies_and_attributes",
			property: filterconfig.MatchProperties{
				LogBodies: []string{},
			},
			errorString: `at least one of "attributes", "libraries", "resources" or "log_bodies" field must be specified`,
		},
		{
			name: "span_properties",
			property: filterconfig.MatchProperties{
				SpanNames: []string{"span"},
			},
			errorString: "neither services nor span_names should be specified for log records",
		},
		{
			name: "invalid_match_type",
			property: filterconfig.MatchProperties{
				Config:     *createConfig("wrong_match_type"),
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "def"}},
			},
			errorString: "error creating attribute filters: unrecognized match_type: 'wrong_match_type', valid types are: [regexp strict]",
		},
		{
			name: "missing_match_type",
			property: filterconfig.MatchProperties{
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "def"}},
			},
			errorString: "error creating attribute filters: unrecognized match_type: '', valid types are: [regexp strict]",
		},
		{
			name: "invalid_regexp_pattern",
			property: filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "["}},
			},
			errorString: "error creating attribute filters: error parsing regexp: missing closing ]: `[`",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := NewMatcher(&tc.property)
			assert.Nil(t, output)
			require.NotNil(t, err)
			assert.Equal(t, tc.errorString, err.Error())
		})
	}
}

func TestLogRecord_Matching_False(t *testing.T) {
	testcases := []struct {
		name       string
		properties *filterconfig.MatchProperties
	}{
		{
			name: "attributes_dont_match",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{
					{Key: "abc", Value: "def"},
				},
			},
		},

		{
			name: "attributes_dont_match_regex",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{
					{Key: "ab.*c", Value: "def"},
				},
			},
		},
	}

	lr := plog.NewLogRecord()
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			matcher, err := NewMatcher(tc.properties)
			assert.Nil(t, err)
			require.NotNil(t, matcher)

			assert.False(t, matcher.MatchLogRecord(lr, pcommon.Resource{}, pcommon.InstrumentationScope{}))
		})
	}
}

func TestLogRecord_Matching_True(t *testing.T) {
	testcases := []struct {
		name       string
		properties *filterconfig.MatchProperties
	}{
		{
			name: "attribute_strict_match",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Strict),
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "def"}},
			},
		},
		{
			name: "attribute_regex_match",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{
					{Key: "abc", Value: "d.f"},
				},
			},
		},
		{
			name: "log_body_regexp_match",
			properties: &filterconfig.MatchProperties{
				Config:    *createConfig(filterset.Regexp),
				LogBodies: []string{"AUTH.*"},
			},
		},
	}

	lr := plog.NewLogRecord()
	lr.Attributes().InsertString("abc", "def")
	lr.Body().SetStringVal("AUTHENTICATION FAILED")

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := NewMatcher(tc.properties)
			assert.NoError(t, err)
			require.NotNil(t, mp)

			assert.NotNil(t, lr)
			assert.True(t, mp.MatchLogRecord(lr, pcommon.Resource{}, pcommon.InstrumentationScope{}))
		})
	}
}
