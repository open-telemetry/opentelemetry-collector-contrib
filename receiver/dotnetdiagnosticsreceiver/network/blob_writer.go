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

// BlobWriter is an interface extracted from blobFileWriter so that it can be
// swapped with a NopBlobWriter.
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

// NopBlobWriter implements BlobWriter but does nothing.
type NopBlobWriter struct{}

func (w *NopBlobWriter) Init() error {
	return nil
}

func (*NopBlobWriter) append([]byte, int) {
}

func (*NopBlobWriter) flush() {
}

// blobFileWriter writes a byte stream to disk, for debugging, troubleshooting,
// and testing. After construction, call Init() to create the specified
// directory, append() to append bytes to the buffer, and flush() to write the
// contents of the buffer to disk. Each time flush() is called, a new file is
// created with the name `msg.%d.bin` where %d is the number of times flush has
// been called. Since we typically want only the latest messages if something
// goes awry, the oldest files are deleted as new ones are written. Use a
// BlobReader to read the resulting files and fake an IPC connection.
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

// Init() creates the directory that will contain the blob files.
func (w *blobFileWriter) Init() error {
	err := w.mkdir(w.dir, 0700)
	if os.IsExist(err) {
		return nil
	}
	return err
}

// append appends the passed-in bytes to the buffer.
func (w *blobFileWriter) append(bytes []byte, n int) {
	w.p = append(w.p, bytes[:n]...)
}

// flush writes the current buffer to disk, deletes the oldest file if
// necessary, and clears the buffer.
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
