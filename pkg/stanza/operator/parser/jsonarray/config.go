// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package jsonarray // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonarray"

import (
	"fmt"
	"strings"

	"github.com/valyala/fastjson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	operatorType    = "json_array_parser"
	headerDelimiter = ","
)

var jsonArrayParserFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"logs.jsonParserArray",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, allows usage of `json_array_parser`."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30321"),
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new json array parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new json array parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a json array parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`
	Header              string `mapstructure:"header"`
}

// Build will build a json array parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	if !jsonArrayParserFeatureGate.IsEnabled() {
		return nil, fmt.Errorf("%s operator disabled", operatorType)
	}

	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.Header != "" {
		return &Parser{
			ParserOperator: parserOperator,
			parse:          generateParseToMapFunc(new(fastjson.ParserPool), strings.Split(c.Header, headerDelimiter)),
		}, nil
	}

	return &Parser{
		ParserOperator: parserOperator,
		parse:          generateParseToArrayFunc(new(fastjson.ParserPool)),
	}, nil
}
