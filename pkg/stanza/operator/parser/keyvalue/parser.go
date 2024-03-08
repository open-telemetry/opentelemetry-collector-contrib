// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package keyvalue // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/keyvalue"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses key value pairs.
type Parser struct {
	helper.ParserOperator
	delimiter     string
	pairDelimiter string
}

// Process will parse an entry for key value pairs.
func (kv *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return kv.ParserOperator.ProcessWith(ctx, entry, kv.parse)
}

// parse will parse a value as key values.
func (kv *Parser) parse(value any) (any, error) {
	switch m := value.(type) {
	case string:
		return kv.parser(m, kv.delimiter, kv.pairDelimiter)
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as key value pairs", value)
	}
}

func (kv *Parser) parser(input string, delimiter string, pairDelimiter string) (map[string]any, error) {
	if input == "" {
		return nil, fmt.Errorf("parse from field %s is empty", kv.ParseFrom.String())
	}

	pairs, err := parseutils.SplitString(input, pairDelimiter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pairs from input: %w", err)
	}

	return parseutils.ParseKeyValuePairs(pairs, delimiter)
}
