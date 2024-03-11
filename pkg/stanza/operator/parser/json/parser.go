// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"

import (
	"context"
	"fmt"

	jsoniter "github.com/json-iterator/go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses JSON.
type Parser struct {
	helper.ParserOperator
	json jsoniter.API
}

// Process will parse an entry for JSON.
func (j *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return j.ParserOperator.ProcessWith(ctx, entry, j.parse)
}

// parse will parse a value as JSON.
func (j *Parser) parse(value any) (any, error) {
	var parsedValue map[string]any
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
