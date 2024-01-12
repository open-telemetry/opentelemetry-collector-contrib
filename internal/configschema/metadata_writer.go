// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"

import (
	"os"
)

type metadataFileWriter struct {
	dirsCreated map[string]struct{}
}

func newMetadataFileWriter() *metadataFileWriter {
	return &metadataFileWriter{
		dirsCreated: map[string]struct{}{},
	}
}

func (w *metadataFileWriter) write(yamlBytes []byte, filename string) error {
	return os.WriteFile(filename, yamlBytes, 0600)
}
