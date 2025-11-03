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

// IsGzipFile checks if a file is of gzip type by reading its header
func IsGzipFile(f *os.File) bool {
	header := make([]byte, len(magicHeader))
	if _, err := f.ReadAt(header, 0); err != nil {
		if errors.Is(err, io.EOF) {
			return false // empty or too short file
		}
		return false
	}

	return bytes.Equal(header, []byte(magicHeader))
}
