// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"
)

const (
	hashSeparator     = byte(0)
	hashNilMarker     = "<nil>"
	hashPropMarker    = "P:"
	hashTagMarker     = "T:"
	hashTagTrueValue  = byte(1)
	hashTagFalseValue = byte(0)
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

// Hash computes a deterministic hash of the dimension update's properties and tags.
func (d *DimensionUpdate) Hash() uint64 {
	h := xxhash.New()

	if len(d.Properties) > 0 {
		_, _ = h.WriteString(hashPropMarker)
		propKeys := make([]string, 0, len(d.Properties))
		for k := range d.Properties {
			propKeys = append(propKeys, k)
		}
		sort.Strings(propKeys)

		for _, k := range propKeys {
			_, _ = h.WriteString(k)
			_, _ = h.Write([]byte{hashSeparator})
			v := d.Properties[k]
			if v != nil {
				_, _ = h.WriteString(*v)
			} else {
				_, _ = h.WriteString(hashNilMarker)
			}
			_, _ = h.Write([]byte{hashSeparator})
		}
	}

	if len(d.Tags) > 0 {
		_, _ = h.WriteString(hashTagMarker)
		tagKeys := make([]string, 0, len(d.Tags))
		for k := range d.Tags {
			tagKeys = append(tagKeys, k)
		}
		sort.Strings(tagKeys)

		for _, k := range tagKeys {
			_, _ = h.WriteString(k)
			_, _ = h.Write([]byte{hashSeparator})
			if d.Tags[k] {
				_, _ = h.Write([]byte{hashTagTrueValue})
			} else {
				_, _ = h.Write([]byte{hashTagFalseValue})
			}
			_, _ = h.Write([]byte{hashSeparator})
		}
	}

	return h.Sum64()
}
