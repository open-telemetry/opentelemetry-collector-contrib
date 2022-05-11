// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package json

import (
	"context"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("json_parser", func() operator.Builder { return NewJSONParserConfig("") })
}

// NewJSONParserConfig creates a new JSON parser config with default values
func NewJSONParserConfig(operatorID string) *JSONParserConfig {
	return &JSONParserConfig{
		ParserConfig: helper.NewParserConfig(operatorID, "json_parser"),
	}
}

// JSONParserConfig is the configuration of a JSON parser operator.
type JSONParserConfig struct {
	helper.ParserConfig `mapstructure:",squash" yaml:",inline"`
}

// Build will build a JSON parser operator.
func (c JSONParserConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &JSONParser{
		ParserOperator: parserOperator,
		json:           jsoniter.ConfigFastest,
	}, nil
}

// JSONParser is an operator that parses JSON.
type JSONParser struct {
	helper.ParserOperator
	json jsoniter.API
}

// Process will parse an entry for JSON.
func (j *JSONParser) Process(ctx context.Context, entry *entry.Entry) error {
	return j.ParserOperator.ProcessWith(ctx, entry, j.parse)
}

// parse will parse a value as JSON.
func (j *JSONParser) parse(value interface{}) (interface{}, error) {
	var parsedValue map[string]interface{}
	switch m := value.(type) {
	case string:
		err := j.json.UnmarshalFromString(m, &parsedValue)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}
	return parsedValue, nil
}
