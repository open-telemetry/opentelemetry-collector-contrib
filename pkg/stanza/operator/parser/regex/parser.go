// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regex // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses regex in an entry.
type Parser struct {
	helper.ParserOperator
	regexp *regexp.Regexp
	cache  cache
}

func (p *Parser) Stop() error {
	if p.cache != nil {
		p.cache.stop()
	}
	return nil
}

// Process will parse an entry for regex.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a value using the supplied regex.
func (p *Parser) parse(value any) (any, error) {
	var raw string
	switch m := value.(type) {
	case string:
		raw = m
	default:
		return nil, fmt.Errorf("type '%T' cannot be parsed as regex", value)
	}
	return p.match(raw)
}

func (p *Parser) match(value string) (any, error) {
	if p.cache != nil {
		if x := p.cache.get(value); x != nil {
			return x, nil
		}
	}

	matches := p.regexp.FindStringSubmatch(value)
	if matches == nil {
		return nil, fmt.Errorf("regex pattern does not match")
	}

	parsedValues := map[string]any{}
	for i, subexp := range p.regexp.SubexpNames() {
		if i == 0 {
			// Skip whole match
			continue
		}
		if subexp != "" {
			parsedValues[subexp] = matches[i]
		}
	}

	if p.cache != nil {
		p.cache.add(value, parsedValues)
	}

	return parsedValues, nil
}
