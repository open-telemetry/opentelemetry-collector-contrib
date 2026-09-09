// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"strings"
)

type DimensionUpdate struct {
	Name       string
	Value      string
	Properties map[string]*string
	Tags       map[string]bool
	// Replace uses the public dimension replacement endpoint
	// (PUT /v2/dimension/{key}/{value}) instead of the sfxagent PATCH endpoint.
	Replace bool
}

func (d *DimensionUpdate) String() string {
	props := "{"
	for k, v := range d.Properties {
		var val string
		if v != nil {
			val = *v
		}
		props += fmt.Sprintf("%v: %q, ", k, val)
	}
	props = strings.Trim(props, ", ") + "}"
	return fmt.Sprintf("{name: %q; value: %q; props: %v; tags: %v; replace: %t}", d.Name, d.Value, props, d.Tags, d.Replace)
}

func (d *DimensionUpdate) Key() DimensionUpdateKey {
	return DimensionUpdateKey{
		Name:    d.Name,
		Value:   d.Value,
		Replace: d.Replace,
	}
}

// DimensionUpdateKey uniquely identifies a queued dimension update.
type DimensionUpdateKey struct {
	Name    string
	Value   string
	Replace bool
}

func (dk DimensionUpdateKey) String() string {
	return fmt.Sprintf("[%s/%s/%t]", dk.Name, dk.Value, dk.Replace)
}
