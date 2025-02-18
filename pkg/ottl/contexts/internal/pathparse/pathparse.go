// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pathparse

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type PathFunc[K any] func(path ottl.Path[K]) (ottl.GetSetter[K], error)

// Parser provides a generic way to parse paths for any context
type Parser[K any] struct {
	telemetrySettings component.TelemetrySettings
	implicitName      string
	implicitDesc      string
	paths             map[string]PathFunc[K]
	defaultErrorFunc  DefaultErrorFunc[K]
}

type DefaultErrorFunc[K any] func(path ottl.Path[K]) error

// NewParser creates a new Parser with the given configuration
func NewParser[K any](
	set component.TelemetrySettings,
	implicitContextName string,
	implicitContextDesc string,
	paths map[string]PathFunc[K],
	defaultErrorFunc DefaultErrorFunc[K],
) *Parser[K] {
	return &Parser[K]{
		telemetrySettings: set,
		implicitName:      implicitContextName,
		implicitDesc:      implicitContextDesc,
		paths:             paths,
		defaultErrorFunc:  defaultErrorFunc,
	}
}

// Parse provides the standard path parsing logic used by all contexts
func (p *Parser[K]) Parse(path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, fmt.Errorf("path cannot be nil")
	}

	pathContext := path.Context()
	if pathContext == "" || pathContext == p.implicitName {
		pathContext = path.Name()
		if next := path.Next(); next != nil {
			path = next
		}
	}

	pathFunc, ok := p.paths[pathContext]
	if ok {
		return pathFunc(path)
	}

	return nil, p.defaultErrorFunc(path)
}
