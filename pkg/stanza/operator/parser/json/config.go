// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"

import (
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "json_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new JSON parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a JSON parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	*JsoniterConfig `mapstructure:"jsoniter_config,omitempty"`
}

type JsoniterConfig struct {
	IndentionStep                 int    `mapstructure:"indention_step"`
	MarshalFloatWith6Digits       bool   `mapstructure:"marshal_float_with_6_digits"`
	EscapeHTML                    bool   `mapstructure:"escape_html"`
	SortMapKeys                   bool   `mapstructure:"sort_map_keys"`
	UseNumber                     bool   `mapstructure:"use_number"`
	DisallowUnknownFields         bool   `mapstructure:"disallow_unknown_fields"`
	TagKey                        string `mapstructure:"tag_key"`
	OnlyTaggedField               bool   `mapstructure:"only_tagged_field"`
	ValidateJsonRawMessage        bool   `mapstructure:"validate_json_raw_message"`
	ObjectFieldMustBeSimpleString bool   `mapstructure:"object_field_must_be_simple_string"`
	CaseSensitive                 bool   `mapstructure:"case_sensitive"`
}

func (jc JsoniterConfig) toJsoniterAPI() jsoniter.API {
	return jsoniter.Config{
		IndentionStep:                 jc.IndentionStep,
		MarshalFloatWith6Digits:       jc.MarshalFloatWith6Digits,
		EscapeHTML:                    jc.EscapeHTML,
		SortMapKeys:                   jc.SortMapKeys,
		UseNumber:                     jc.UseNumber,
		DisallowUnknownFields:         jc.DisallowUnknownFields,
		TagKey:                        jc.TagKey,
		OnlyTaggedField:               jc.OnlyTaggedField,
		ValidateJsonRawMessage:        jc.ValidateJsonRawMessage,
		ObjectFieldMustBeSimpleString: jc.ObjectFieldMustBeSimpleString,
		CaseSensitive:                 jc.CaseSensitive,
	}.Froze()
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	var json jsoniter.API
	var convertNumber bool
	if c.JsoniterConfig != nil {
		json = c.JsoniterConfig.toJsoniterAPI()
		convertNumber = c.JsoniterConfig.UseNumber
	} else {
		json = jsoniter.ConfigFastest
	}

	return &Parser{
		ParserOperator: parserOperator,
		json:           json,
		useNumber:      convertNumber,
	}, nil
}
