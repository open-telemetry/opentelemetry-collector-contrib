// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// fields represents metadata
type fields struct {
	orig     pcommon.Map
	replacer *strings.Replacer
}

func newFields(attrMap pcommon.Map) fields {
	return fields{
		orig:     attrMap,
		replacer: strings.NewReplacer(",", "_", "=", ":", "\n", "_"),
	}
}

// string returns fields as ordered key=value string with `, ` as separator
func (f fields) string() string {
	returnValue := make([]string, 0, f.orig.Len())
	f.orig.Range(func(k string, v pcommon.Value) bool {
		returnValue = append(
			returnValue,
			fmt.Sprintf(
				"%s=%s",
				f.sanitizeField(k),
				f.sanitizeField(v.AsString()),
			),
		)
		return true
	})
	sort.Strings(returnValue)

	return strings.Join(returnValue, ", ")
}

// sanitizeFields sanitize field (key or value) to be correctly parsed by sumologic receiver
func (f fields) sanitizeField(fld string) string {
	return f.replacer.Replace(fld)
}
