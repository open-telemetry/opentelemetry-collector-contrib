// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/sampler"
)

// rule is a compiled rule: a list of conditions plus the sampler to invoke
// when all conditions match.
type rule struct {
	name       string
	conditions []condition
	sampler    sampler.Sampler
	// samplerConfig retains the wire-level sampler config so we can rebuild key
	// fields without re-parsing. Used by the EMA sampler for key extraction.
	keyFields []string
}

// matches returns true when every condition matches the trace data.
func (r *rule) matches(spans []ptrace.ResourceSpans) bool {
	for _, c := range r.conditions {
		if !c.matches(spans) {
			return false
		}
	}
	return true
}

// condition is an evaluable predicate over an accumulated trace.
type condition interface {
	matches(spans []ptrace.ResourceSpans) bool
}

// statusCodeCondition matches when at least one span has the given status code.
// Status codes use the same values as the OTel status code enum: 0 (Unset), 1
// (Ok), 2 (Error).
type statusCodeCondition struct {
	code    int
	negated bool
}

func (c statusCodeCondition) matches(spans []ptrace.ResourceSpans) bool {
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				if int(span.Status().Code()) == c.code {
					return !c.negated
				}
			}
		}
	}
	return c.negated
}

// attributeEqualsCondition matches when any span (or its resource) has an
// attribute with the configured value. Negated form returns true when no
// matching value is found.
type attributeEqualsCondition struct {
	field   string
	value   string
	negated bool
}

func (c attributeEqualsCondition) matches(spans []ptrace.ResourceSpans) bool {
	for _, rs := range spans {
		if v, ok := rs.Resource().Attributes().Get(c.field); ok && v.AsString() == c.value {
			return !c.negated
		}
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				if v, ok := span.Attributes().Get(c.field); ok && v.AsString() == c.value {
					return !c.negated
				}
			}
		}
	}
	return c.negated
}

// parseCondition parses a simple condition string. Supported forms:
//
//	status.code == N
//	status.code != N
//	field == "value"   (or 'value', or bareword)
//	field != "value"
//
// More complex expressions will be supported in a future PR via OTTL.
func parseCondition(expr string) (condition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty condition")
	}

	negated := false
	switch {
	case strings.Contains(expr, "!="):
		i := strings.Index(expr, "!=")
		negated = true
		expr = strings.TrimSpace(expr[:i]) + "\x00" + strings.TrimSpace(expr[i+2:])
	case strings.Contains(expr, "=="):
		i := strings.Index(expr, "==")
		expr = strings.TrimSpace(expr[:i]) + "\x00" + strings.TrimSpace(expr[i+2:])
	default:
		return nil, fmt.Errorf("condition %q: expected `==` or `!=`", expr)
	}

	parts := strings.SplitN(expr, "\x00", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("condition: malformed expression")
	}
	field := parts[0]
	raw := strings.Trim(parts[1], `"' `)

	if field == "status.code" {
		code, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("condition: status.code value %q is not an integer", raw)
		}
		return statusCodeCondition{code: code, negated: negated}, nil
	}

	return attributeEqualsCondition{field: field, value: raw, negated: negated}, nil
}

// compileRule turns a config rule into a runtime rule. The sampler must be
// supplied by the caller because constructing it depends on processor-wide
// resources.
func compileRule(cfg RuleConfig, s sampler.Sampler, keyFields []string) (*rule, error) {
	r := &rule{name: cfg.Name, sampler: s, keyFields: keyFields}
	for i, expr := range cfg.Conditions {
		c, err := parseCondition(expr)
		if err != nil {
			return nil, fmt.Errorf("rule %q condition[%d]: %w", cfg.Name, i, err)
		}
		r.conditions = append(r.conditions, c)
	}
	return r, nil
}
