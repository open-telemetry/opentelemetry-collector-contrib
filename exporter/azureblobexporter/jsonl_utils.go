// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"bytes"
	"regexp"
)

func removeNullValues(data []byte) []byte {
	if !bytes.Contains(data, []byte(": null")) {
		return data
	}
	
	nullFieldRegex := regexp.MustCompile(`"[^"]+"\s*:\s*null\s*,?\s*`)
	result := nullFieldRegex.ReplaceAll(data, []byte(""))
	
	result = regexp.MustCompile(`,\s*}`).ReplaceAll(result, []byte("}"))
	result = regexp.MustCompile(`,\s*]`).ReplaceAll(result, []byte("]"))
	
	return result
}