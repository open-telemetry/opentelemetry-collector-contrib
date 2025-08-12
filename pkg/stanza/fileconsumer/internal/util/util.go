// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"

import (
	"bytes"
	"errors"
	"io"
	"os"
)

const magicHeader = "\x1f\x8b" // RFC 1952 magic bytes

// MapCopy deep copies the provided attributes map.
func MapCopy(m map[string]any) map[string]any {
	newMap := make(map[string]any, len(m))
	for k, v := range m {
		switch typedVal := v.(type) {
		case map[string]any:
			newMap[k] = MapCopy(typedVal)
		default:
			// Assume any other values are safe to directly copy.
			// Struct types and slice types shouldn't appear in attribute maps from pipelines
			newMap[k] = v
		}
	}
	return newMap
}

// IsGzipFile checks if a file is of gzip type by reading its header
func IsGzipFile(f *os.File) bool {
	header := make([]byte, len(magicHeader))
	if _, err := f.ReadAt(header, 0); err != nil {
		if errors.Is(err, io.EOF) {
			return false // empty or too short file – definitely not gzip
		}
		return false
	}

	return bytes.Equal(header, []byte(magicHeader))
}
