// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"io/ioutil"
	"os"

	"go.uber.org/zap"
)

// BlobWriter
type BlobWriter interface {
	Init() error
	append(bytes []byte, n int)
	flush()
}

func NewBlobWriter(blobDir string, maxBlobFiles int, logger *zap.Logger) BlobWriter {
	if blobDir == "" {
		return &NopBlobWriter{}
	}
	return newBlobFileWriter(blobDir, maxBlobFiles, logger)
}

// NopBlobWriter
type NopBlobWriter struct{}

func (*NopBlobWriter) append([]byte, int) {
}

func (*NopBlobWriter) flush() {
}

func (w *NopBlobWriter) Init() error {
	return nil
}

// blobFileWriter
type blobFileWriter struct {
	logger   *zap.Logger
	dir      string
	maxFiles int

	p []byte
	i int

	mkdir     func(string, os.FileMode) error
	remove    func(string) error
	writeFile func(string, []byte, os.FileMode) error
}

func newBlobFileWriter(dir string, maxFiles int, logger *zap.Logger) *blobFileWriter {
	return &blobFileWriter{
		logger:    logger,
		dir:       dir,
		maxFiles:  maxFiles,
		mkdir:     os.Mkdir,
		remove:    os.Remove,
		writeFile: ioutil.WriteFile,
	}
}

func (w *blobFileWriter) Init() error {
	err := w.mkdir(w.dir, 0755)
	if os.IsExist(err) {
		return nil
	}
	return err
}

func (w *blobFileWriter) append(bytes []byte, n int) {
	w.p = append(w.p, bytes[:n]...)
}

func (w *blobFileWriter) flush() {
	w.deleteOldestFile()
	w.writeCurrentFile()
	w.p = nil
	w.i++
}

func (w *blobFileWriter) deleteOldestFile() {
	if w.i < w.maxFiles {
		return
	}
	oldestIdx := w.i - w.maxFiles
	f := blobFile(w.dir, oldestIdx)
	err := w.remove(f)
	if err != nil {
		w.logger.Warn(
			"error deleting blob",
			zap.String("filename", f),
			zap.Error(err),
		)
	}
}

func (w *blobFileWriter) writeCurrentFile() {
	f := blobFile(w.dir, w.i)
	err := w.writeFile(f, w.p, 0600)
	if err != nil {
		w.logger.Warn(
			"error writing blob file",
			zap.String("filename", f),
			zap.Error(err),
		)
	}
}
