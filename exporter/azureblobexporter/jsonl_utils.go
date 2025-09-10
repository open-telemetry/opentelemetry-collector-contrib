// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"bytes"
	"regexp"
)

// removeNullValues removes null values from JSON to prevent Jaeger crashes
// This is a defensive measure against null attributes that can cause NullPointerException in Jaeger
func removeNullValues(data []byte) []byte {
	// Quick check - if no null values, return as is
	if !bytes.Contains(data, []byte(": null")) {
		return data
	}
	
	// Use regex to remove null fields: "field": null, or "field": null}
	// This handles the production case: "http.status_code": null, "net.host.port": null, etc.
	nullFieldRegex := regexp.MustCompile(`"[^"]+"\s*:\s*null\s*,?\s*`)
	
	result := nullFieldRegex.ReplaceAll(data, []byte(""))
	
	// Clean up any trailing commas that might result from removals
	result = regexp.MustCompile(`,\s*}`).ReplaceAll(result, []byte("}"))
	result = regexp.MustCompile(`,\s*]`).ReplaceAll(result, []byte("]"))
	
	return result
}