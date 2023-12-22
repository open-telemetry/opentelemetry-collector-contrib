// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"fmt"
	"os"
	"path/filepath"
)

type metadataWriter interface {
	write(cfg CfgInfo, bytes []byte) error
}

type metadataFileWriter struct {
	baseDir     string
	dirsCreated map[string]struct{}
}

func newMetadataFileWriter(dir string) metadataWriter {
	return &metadataFileWriter{
		dirsCreated: map[string]struct{}{},
		baseDir:     dir,
	}
}

func (w *metadataFileWriter) write(cfg CfgInfo, yamlBytes []byte) error {
	filename := filepath.Join(w.baseDir, fmt.Sprintf("%s.yaml", cfg.Type))
	fmt.Printf("writing file: %s\n", filename)
	return os.WriteFile(filename, yamlBytes, 0600)
}

func (w *metadataFileWriter) prepDir(dir string) error {
	if _, ok := w.dirsCreated[dir]; !ok {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to make dir %q: %w", dir, err)
		}
		w.dirsCreated[dir] = struct{}{}
	}
	return nil
}
