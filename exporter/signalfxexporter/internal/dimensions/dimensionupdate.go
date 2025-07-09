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
	return fmt.Sprintf("{name: %q; value: %q; props: %v; tags: %v}", d.Name, d.Value, props, d.Tags)
}

func (d *DimensionUpdate) Key() DimensionKey {
	return DimensionKey{
		Name:  d.Name,
		Value: d.Value,
	}
}

// DimensionKey is what uniquely identifies a dimension, its name and value
// together.
type DimensionKey struct {
	Name  string
	Value string
}

func (dk DimensionKey) String() string {
	return fmt.Sprintf("[%s/%s]", dk.Name, dk.Value)
}
