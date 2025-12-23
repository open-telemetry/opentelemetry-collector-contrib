// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/compression"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
)

const gzipHeader = "\x1f\x8b" // RFC 1952 magic bytes

// IsGzipFile checks if a file is of gzip type by reading its header
func IsGzipFile(f *os.File, logger *zap.Logger) bool {
	header := make([]byte, len(gzipHeader))
	if _, err := f.ReadAt(header, 0); err != nil {
		if errors.Is(err, io.EOF) {
			return false // empty or too short file
		}

		logger.Error(fmt.Sprintf("error reading file: %s: %s", f.Name(), err))
		return false
	}

	return bytes.Equal(header, []byte(gzipHeader))
}
