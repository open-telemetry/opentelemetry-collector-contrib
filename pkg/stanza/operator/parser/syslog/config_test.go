// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package syslog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:      "default",
				Expect:    NewConfig(),
				ExpectErr: false, // missing protocol, caught later by Config.Validate()
			},
			{
				Name: "rfc3164",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC3164
					return cfg
				}(),
			},
			{
				Name: "rfc5424",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					return cfg
				}(),
			},
			{
				Name: "location",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.Location = "FOO"
					return cfg
				}(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "parse_from_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.ParseFrom = entry.NewBodyField("from")
					return cfg
				}(),
			},
			{
				Name: "parse_to_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField("log")}
					return cfg
				}(),
			},
			{
				Name: "timestamp",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					parseField := entry.NewBodyField("timestamp_field")
					newTime := helper.TimeParser{
						LayoutType: "strptime",
						Layout:     "%Y-%m-%d",
						ParseFrom:  &parseField,
					}
					cfg.TimeParser = &newTime
					return cfg
				}(),
			},
			{
				Name: "severity",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					parseField := entry.NewBodyField("severity_field")
					severityParser := helper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					mapping := map[string]any{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					severityParser.Mapping = mapping
					cfg.SeverityConfig = &severityParser
					return cfg
				}(),
			},
			{
				Name: "scope_name",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					loggerNameParser := helper.NewScopeNameParser()
					loggerNameParser.ParseFrom = entry.NewBodyField("logger_name_field")
					cfg.ScopeNameParser = &loggerNameParser
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func TestParserMissingProtocol(t *testing.T) {
	_, err := NewConfig().Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing field 'protocol'")
}

func TestRFC6587ConfigOptions(t *testing.T) {
	validFramingTrailer := NULTrailer
	invalidFramingTrailer := "bad"
	testCases := []struct {
		desc        string
		cfg         *Config
		errContents string
	}{
		{
			desc: "Octet Counting with RFC3164",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:            RFC3164,
					EnableOctetCounting: true,
				},
			},
			errContents: "octet_counting and non_transparent_framing are only compatible with protocol rfc5424",
		},
		{
			desc: "Non-Transparent-Framing with RFC3164",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:                     RFC3164,
					NonTransparentFramingTrailer: &validFramingTrailer,
				},
			},
			errContents: "octet_counting and non_transparent_framing are only compatible with protocol rfc5424",
		},
		{
			desc: "Non-Transparent-Framing and Octet counting both enabled with RFC5424",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:                     RFC5424,
					NonTransparentFramingTrailer: &validFramingTrailer,
					EnableOctetCounting:          true,
				},
			},
			errContents: "only one of octet_counting or non_transparent_framing can be enabled",
		},
		{
			desc: "Valid Octet Counting",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:                     RFC5424,
					NonTransparentFramingTrailer: nil,
					EnableOctetCounting:          true,
				},
			},
			errContents: "",
		},
		{
			desc: "Valid Non-Transparent-Framing Trailer",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:                     RFC5424,
					NonTransparentFramingTrailer: &validFramingTrailer,
					EnableOctetCounting:          false,
				},
			},
			errContents: "",
		},
		{
			desc: "Invalid Non-Transparent-Framing Trailer",
			cfg: &Config{
				ParserConfig: helper.NewParserConfig(operatorType, operatorType),
				BaseConfig: BaseConfig{
					Protocol:                     RFC5424,
					NonTransparentFramingTrailer: &invalidFramingTrailer,
					EnableOctetCounting:          false,
				},
			},
			errContents: "invalid non_transparent_framing_trailer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := tc.cfg.Build(testutil.Logger(t))
			if tc.errContents != "" {
				require.ErrorContains(t, err, tc.errContents)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParserInvalidLocation(t *testing.T) {
	config := NewConfig()
	config.Location = "not_a_location"
	config.Protocol = RFC3164

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location "+config.Location)
}
