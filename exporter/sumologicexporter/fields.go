// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/exp/slices"
)

// fields represents metadata
type fields struct {
	orig        pcommon.Map
	initialized bool
}

func newFields(attrMap pcommon.Map) fields {
	return fields{
		orig:        attrMap,
		initialized: true,
	}
}

func (f fields) isInitialized() bool {
	return f.initialized
}

// string returns fields as ordered key=value string with `, ` as separator
func (f fields) string() string {
	if !f.initialized {
		return ""
	}

	returnValue := make([]string, 0, f.orig.Len())

	for k, v := range f.orig.All() {
		// Don't add source related attributes to fields as they are handled separately
		// and are added to the payload either as special HTTP headers or as resources
		// attributes.
		if k == attributeKeySourceCategory || k == attributeKeySourceHost || k == attributeKeySourceName {
			continue
		}

		sv := v.AsString()

		// Skip empty field
		if len(sv) == 0 {
			continue
		}

		key := []byte(k)
		f.sanitizeField(key)
		value := []byte(sv)
		f.sanitizeField(value)
		sb := strings.Builder{}
		sb.Grow(len(key) + len(value) + 1)
		sb.Write(key)
		sb.WriteRune('=')
		sb.Write(value)

		returnValue = append(
			returnValue,
			sb.String(),
		)
	}
	slices.Sort(returnValue)

	return strings.Join(returnValue, ", ")
}

// sanitizeFields sanitize field (key or value) to be correctly parsed by sumologic receiver
// It modifies the field in place.
func (f fields) sanitizeField(fld []byte) {
	for i := 0; i < len(fld); i++ {
		switch fld[i] {
		case ',':
			fld[i] = '_'
		case '=':
			fld[i] = ':'
		case '\n':
			fld[i] = '_'
		default:
		}
	}
}
