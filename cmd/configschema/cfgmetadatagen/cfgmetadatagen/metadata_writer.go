// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgmetadatagen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

type metadataWriter interface {
	write(cfg configschema.CfgInfo, bytes []byte) error
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

func (w *metadataFileWriter) write(cfg configschema.CfgInfo, yamlBytes []byte) error {
	groupDir := filepath.Join(w.baseDir, cfg.Group)
	if err := w.prepDir(groupDir); err != nil {
		return err
	}
	filename := filepath.Join(groupDir, fmt.Sprintf("%s.yaml", cfg.Type))
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
