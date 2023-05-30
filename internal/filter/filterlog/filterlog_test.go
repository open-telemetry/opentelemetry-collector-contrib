// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
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
			errorString: filterconfig.ErrMissingRequiredLogField.Error(),
		},
		{
			name: "empty_log_bodies_and_attributes",
			property: filterconfig.MatchProperties{
				LogBodies:        []string{},
				LogSeverityTexts: []string{},
			},
			errorString: filterconfig.ErrMissingRequiredLogField.Error(),
		},
		{
			name: "span_properties",
			property: filterconfig.MatchProperties{
				SpanNames: []string{"span"},
			},
			errorString: filterconfig.ErrInvalidLogField.Error(),
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
			expr, err := newExpr(&tc.property)
			assert.Nil(t, expr)
			require.NotNil(t, err)
			println(tc.name)
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
		{
			name: "log_severity_text_regexp_dont_match",
			properties: &filterconfig.MatchProperties{
				Config:           *createConfig(filterset.Regexp),
				LogSeverityTexts: []string{"debug.*"},
			},
		},
		{
			name: "log_min_severity_trace_dont_match",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
					Min: plog.SeverityNumberInfo,
				},
			},
		},
		{
			name: "log_body_doesnt_match",
			properties: &filterconfig.MatchProperties{
				Config:    *createConfig(filterset.Regexp),
				LogBodies: []string{".*TEST.*"},
			},
		},
	}

	lr := plog.NewLogRecord()
	lr.SetSeverityNumber(plog.SeverityNumberTrace)
	lr.Body().SetStr("AUTHENTICATION FAILED")

	lrm := plog.NewLogRecord()
	lrm.Body().SetEmptyMap()
	lrm.Body().Map().PutStr("message", "AUTHENTICATION FAILED")

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := newExpr(tc.properties)
			assert.Nil(t, err)
			require.NotNil(t, expr)

			val, err := expr.Eval(context.Background(), ottllog.NewTransformContext(lr, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			require.NoError(t, err)
			assert.False(t, val)

			val, err = expr.Eval(context.Background(), ottllog.NewTransformContext(lrm, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			require.NoError(t, err)
			assert.False(t, val)
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
		{
			name: "log_severity_text_regexp_match",
			properties: &filterconfig.MatchProperties{
				Config:           *createConfig(filterset.Regexp),
				LogSeverityTexts: []string{"debug.*"},
			},
		},
		{
			name: "log_min_severity_match",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
					Min: plog.SeverityNumberDebug,
				},
			},
		},
	}

	lr := plog.NewLogRecord()
	lr.Attributes().PutStr("abc", "def")
	lr.Body().SetStr("AUTHENTICATION FAILED")
	lr.SetSeverityText("debug")
	lr.SetSeverityNumber(plog.SeverityNumberDebug)

	lrm := plog.NewLogRecord()
	lrm.Attributes().PutStr("abc", "def")
	lrm.Body().SetEmptyMap()
	lrm.Body().Map().PutStr("message", "AUTHENTICATION FAILED")
	lrm.SetSeverityText("debug")
	lrm.SetSeverityNumber(plog.SeverityNumberDebug)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := newExpr(tc.properties)
			assert.NoError(t, err)
			require.NotNil(t, expr)

			assert.NotNil(t, lr)
			val, err := expr.Eval(context.Background(), ottllog.NewTransformContext(lr, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			require.NoError(t, err)
			assert.True(t, val)

			assert.NotNil(t, lrm)
			val, err = expr.Eval(context.Background(), ottllog.NewTransformContext(lrm, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			require.NoError(t, err)
			assert.True(t, val)
		})
	}
}
