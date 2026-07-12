// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/otelcol"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "otelcol"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new otelcol parser config with default values.
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new otelcol parser config with default values.
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
		Format:       formatAuto,
	}
}

// Config is the configuration of an otelcol parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	// Format is one of "auto", "json", "console". Defaults to "auto".
	Format string `mapstructure:"format"`
}

// Build will build an otelcol parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	format := c.Format
	if format == "" {
		format = formatAuto
	}

	switch format {
	case formatAuto, formatJSON, formatConsole:
	default:
		return nil, fmt.Errorf(
			"operator config has an invalid `format` field %q, must be one of `auto`, `json`, `console`",
			c.Format,
		)
	}

	return &Parser{
		ParserOperator: parserOperator,
		format:         format,
	}, nil
}
