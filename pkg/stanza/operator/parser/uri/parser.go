// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package uri // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/uri"

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses a uri.
type Parser struct {
	helper.ParserOperator
}

// Process will parse an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a uri from a field and attach it to an entry.
func (p *Parser) parse(value any) (any, error) {
	switch m := value.(type) {
	case string:
		return parseURI(m)
	default:
		return nil, fmt.Errorf("type '%T' cannot be parsed as URI", value)
	}
}

// parseURI takes an absolute or relative uri and returns the parsed values.
func parseURI(value string) (map[string]any, error) {
	m := make(map[string]any)

	if strings.HasPrefix(value, "?") {
		// remove the query string '?' prefix before parsing
		v, err := url.ParseQuery(value[1:])
		if err != nil {
			return nil, err
		}
		return queryToMap(v, m), nil
	}

	x, err := url.ParseRequestURI(value)
	if err != nil {
		return nil, err
	}
	return urlToMap(x, m), nil
}

// urlToMap converts a url.URL to a map, excludes any values that are not set.
func urlToMap(p *url.URL, m map[string]any) map[string]any {
	scheme := p.Scheme
	if scheme != "" {
		m["scheme"] = scheme
	}

	user := p.User.Username()
	if user != "" {
		m["user"] = user
	}

	host := p.Hostname()
	if host != "" {
		m["host"] = host
	}

	port := p.Port()
	if port != "" {
		m["port"] = port
	}

	path := p.EscapedPath()
	if path != "" {
		m["path"] = path
	}

	return queryToMap(p.Query(), m)
}

// queryToMap converts a query string url.Values to a map.
func queryToMap(query url.Values, m map[string]any) map[string]any {
	// no-op if query is empty, do not create the key m["query"]
	if len(query) == 0 {
		return m
	}

	/* 'parameter' will represent url.Values
	map[string]any{
		"parameter-a": []any{
			"a",
			"b",
		},
		"parameter-b": []any{
			"x",
			"y",
		},
	}
	*/
	parameters := map[string]any{}
	for param, values := range query {
		parameters[param] = queryParamValuesToMap(values)
	}
	m["query"] = parameters
	return m
}

// queryParamValuesToMap takes query string parameter values and
// returns an []interface populated with the values
func queryParamValuesToMap(values []string) []any {
	v := make([]any, len(values))
	for i, value := range values {
		v[i] = value
	}
	return v
}
