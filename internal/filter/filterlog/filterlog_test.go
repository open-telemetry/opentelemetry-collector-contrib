// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func createConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}

func TestLogRecord_validateMatchesConfiguration_InvalidConfig(t *testing.T) {
	testcases := []struct {
		name        string
		property    *filterconfig.MatchProperties
		errorString string
	}{
		{
			name:        "empty_property",
			property:    &filterconfig.MatchProperties{},
			errorString: filterconfig.ErrMissingRequiredLogField.Error(),
		},
		{
			name: "empty_log_bodies_and_attributes",
			property: &filterconfig.MatchProperties{
				LogBodies:        []string{},
				LogSeverityTexts: []string{},
			},
			errorString: filterconfig.ErrMissingRequiredLogField.Error(),
		},
		{
			name: "span_properties",
			property: &filterconfig.MatchProperties{
				SpanNames: []string{"span"},
			},
			errorString: filterconfig.ErrInvalidLogField.Error(),
		},
		{
			name: "invalid_match_type",
			property: &filterconfig.MatchProperties{
				Config:     *createConfig("wrong_match_type"),
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "def"}},
			},
			errorString: "error creating attribute filters: unrecognized match_type: 'wrong_match_type', valid types are: [regexp strict]",
		},
		{
			name: "missing_match_type",
			property: &filterconfig.MatchProperties{
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "def"}},
			},
			errorString: "error creating attribute filters: unrecognized match_type: '', valid types are: [regexp strict]",
		},
		{
			name: "invalid_regexp_pattern",
			property: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{{Key: "abc", Value: "["}},
			},
			errorString: "error creating attribute filters: error parsing regexp: missing closing ]: `[`",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := newExpr(tc.property)
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

func Test_NewSkipExpr_With_Bridge(t *testing.T) {
	tests := []struct {
		name        string
		condition   *filterconfig.MatchConfig
		logSeverity plog.SeverityNumber
	}{
		// Body
		{
			name: "single static body include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogBodies: []string{"body"},
				},
			},
		},
		{
			name: "multiple static body include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogBodies: []string{"hand", "foot"},
				},
			},
		},
		{
			name: "single regex body include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogBodies: []string{"bod.*"},
				},
			},
		},
		{
			name: "multiple regex body include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogBodies: []string{"hand.*", "foot.*"},
				},
			},
		},
		{
			name: "single static body exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogBodies: []string{"body"},
				},
			},
		},
		{
			name: "multiple static body exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogBodies: []string{"hand", "foot"},
				},
			},
		},
		{
			name: "single regex body exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogBodies: []string{"bod.*"},
				},
			},
		},
		{
			name: "multiple regex body exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogBodies: []string{"hand.*", "foot.*"},
				},
			},
		},

		// Severity text
		{
			name: "single static severity text include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityTexts: []string{"severity text"},
				},
			},
		},
		{
			name: "multiple static severity text include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityTexts: []string{"not", "correct"},
				},
			},
		},
		{
			name: "single regex severity text include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogSeverityTexts: []string{"severity.*"},
				},
			},
		},
		{
			name: "multiple regex severity text include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogSeverityTexts: []string{"not.*", "correct.*"},
				},
			},
		},
		{
			name: "single static severity text exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityTexts: []string{"severity text"},
				},
			},
		},
		{
			name: "multiple static severity text exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityTexts: []string{"not", "correct"},
				},
			},
		},
		{
			name: "single regex severity text exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogSeverityTexts: []string{"severity.*"},
				},
			},
		},
		{
			name: "multiple regex severity text exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogSeverityTexts: []string{"not.*", "correct.*"},
				},
			},
		},

		// Severity number
		{
			name:        "severity number unspecified, match unspecified true, min unspecified, include",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberUnspecified,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified true, min info, include",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified false, min unspecified, include",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberUnspecified,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified false, min info, include",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified true, min info, include",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified true, min fatal, include",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberFatal,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified false, min info, include",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified false, min fatal, include",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberFatal,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified true, min unspecified, exclude",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberUnspecified,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified true, min info, exclude",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified false, min unspecified, exclude",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberUnspecified,
					},
				},
			},
		},
		{
			name:        "severity number unspecified, match unspecified false, min info, exclude",
			logSeverity: plog.SeverityNumberUnspecified,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified true, min info, exclude",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified true, min fatal, exclude",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: true,
						Min:            plog.SeverityNumberFatal,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified false, min info, exclude",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberInfo,
					},
				},
			},
		},
		{
			name:        "severity number not unspecified, match unspecified false, min fatal, exclude",
			logSeverity: plog.SeverityNumberInfo,
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberFatal,
					},
				},
			},
		},

		// Scope name
		{
			name: "single static scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple static scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo",
						},
						{
							Name: "bar",
						},
					},
				},
			},
		},
		{
			name: "single regex scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo.*",
						},
						{
							Name: "bar.*",
						},
					},
				},
			},
		},
		{
			name: "single static scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple static scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo",
						},
						{
							Name: "bar",
						},
					},
				},
			},
		},
		{
			name: "single regex scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo.*",
						},
						{
							Name: "bar.*",
						},
					},
				},
			},
		},

		// Scope version
		{
			name: "single static scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
				},
			},
		},
		{
			name: "multiple static scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.0.0"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1.1.0`),
						},
					},
				},
			},
		},
		{
			name: "single regex scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.*"),
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.*"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp("^1\\\\.1.*"),
						},
					},
				},
			},
		},
		{
			name: "single static scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
				},
			},
		},
		{
			name: "multiple static scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.0.0"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1.1.0`),
						},
					},
				},
			},
		},
		{
			name: "single regex scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.*"),
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.*"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1\\.1.*`),
						},
					},
				},
			},
		},

		// attributes
		{
			name: "single static attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
		{
			name: "multiple static attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val2",
						},
						{
							Key:   "attr2",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "single regex attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val",
						},
						{
							Key:   "attr3",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "single static attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
		{
			name: "multiple static attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val2",
						},
						{
							Key:   "attr2",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "single regex attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val",
						},
						{
							Key:   "attr3",
							Value: "val.*",
						},
					},
				},
			},
		},

		// resource attributes
		{
			name: "single static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},
		{
			name: "single static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},

		// complex
		{
			name:        "complex",
			logSeverity: plog.SeverityNumberDebug,
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					LogBodies:        []string{"body"},
					LogSeverityTexts: []string{"severity text"},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						MatchUndefined: false,
						Min:            plog.SeverityNumberInfo,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := plog.NewLogRecord()
			log.Body().SetStr("body")
			log.Attributes().PutStr("keyString", "arithmetic")
			log.Attributes().PutInt("keyInt", 123)
			log.Attributes().PutDouble("keyDouble", 3245.6)
			log.Attributes().PutBool("keyBool", true)
			log.Attributes().PutStr("keyExists", "present")
			log.SetSeverityText("severity text")
			log.SetSeverityNumber(tt.logSeverity)

			resource := pcommon.NewResource()
			resource.Attributes().PutStr(conventions.AttributeServiceName, "svcA")

			scope := pcommon.NewInstrumentationScope()

			tCtx := ottllog.NewTransformContext(log, scope, resource)

			boolExpr, err := NewSkipExpr(tt.condition)
			require.NoError(t, err)
			expectedResult, err := boolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			ottlBoolExpr, err := filterottl.NewLogSkipExprBridge(tt.condition)
			assert.NoError(t, err)
			ottlResult, err := ottlBoolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			assert.Equal(t, expectedResult, ottlResult)
		})
	}
}

func BenchmarkFilterlog_NewSkipExpr(b *testing.B) {
	testCases := []struct {
		name string
		mc   *filterconfig.MatchConfig
		skip bool
	}{
		{
			name: "body_match_regexp",
			mc: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config:    *createConfig(filterset.Regexp),
					LogBodies: []string{"body"},
				},
			},
			skip: false,
		},
		{
			name: "body_match_static",
			mc: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config:    *createConfig(filterset.Strict),
					LogBodies: []string{"body"},
				},
			},
			skip: false,
		},
		{
			name: "severity_number_match",
			mc: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: *createConfig(filterset.Strict),
					LogSeverityNumber: &filterconfig.LogSeverityNumberMatchProperties{
						Min:            plog.SeverityNumberInfo,
						MatchUndefined: true,
					},
				},
			},
			skip: false,
		},
	}

	for _, tt := range testCases {
		origVal := useOTTLBridge.IsEnabled()
		err := featuregate.GlobalRegistry().Set("filter.filterlog.useOTTLBridge", true)
		assert.NoError(b, err)

		skipExpr, err := NewSkipExpr(tt.mc)
		assert.NoError(b, err)

		log := plog.NewLogRecord()
		log.Body().SetStr("body")
		log.SetSeverityNumber(plog.SeverityNumberUnspecified)

		resource := pcommon.NewResource()

		scope := pcommon.NewInstrumentationScope()

		tCtx := ottllog.NewTransformContext(log, scope, resource)

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var skip bool
				skip, err = skipExpr.Eval(context.Background(), tCtx)
				assert.NoError(b, err)
				assert.Equal(b, tt.skip, skip)
			}
		})

		err = featuregate.GlobalRegistry().Set("filter.filterlog.useOTTLBridge", origVal)
		assert.NoError(b, err)
	}
}
