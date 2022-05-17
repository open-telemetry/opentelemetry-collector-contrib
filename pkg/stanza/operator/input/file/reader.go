// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// File attributes contains information about file paths
type fileAttributes struct {
	Name         string
	Path         string
	ResolvedName string
	ResolvedPath string
}

// resolveFileAttributes resolves file attributes
// and sets it to empty string in case of error
func (f *InputOperator) resolveFileAttributes(path string) *fileAttributes {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		f.Error(err)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		f.Error(err)
	}

	return &fileAttributes{
		Path:         path,
		Name:         filepath.Base(path),
		ResolvedPath: abs,
		ResolvedName: filepath.Base(abs),
	}
}

// Reader manages a single file
type Reader struct {
	Fingerprint *Fingerprint
	Offset      int64

	generation     int
	fileInput      *InputOperator
	file           *os.File
	fileAttributes *fileAttributes

	decoder      *encoding.Decoder
	decodeBuffer []byte

	splitter *helper.Splitter

	*zap.SugaredLogger `json:"-"`
}

// NewReader creates a new file reader
func (f *InputOperator) NewReader(path string, file *os.File, fp *Fingerprint, splitter *helper.Splitter) (*Reader, error) {
	r := &Reader{
		Fingerprint:    fp,
		file:           file,
		fileInput:      f,
		SugaredLogger:  f.SugaredLogger.With("path", path),
		decoder:        f.encoding.Encoding.NewDecoder(),
		decodeBuffer:   make([]byte, 1<<12),
		fileAttributes: f.resolveFileAttributes(path),
		splitter:       splitter,
	}
	return r, nil
}

// Copy creates a deep copy of a Reader
func (r *Reader) Copy(file *os.File) (*Reader, error) {
	reader, err := r.fileInput.NewReader(r.fileAttributes.Path, file, r.Fingerprint.Copy(), r.splitter)
	if err != nil {
		return nil, err
	}
	reader.Offset = r.Offset
	return reader, nil
}

// InitializeOffset sets the starting offset
func (r *Reader) InitializeOffset(startAtBeginning bool) error {
	if !startAtBeginning {
		info, err := r.file.Stat()
		if err != nil {
			return fmt.Errorf("stat: %s", err)
		}
		r.Offset = info.Size()
	}
	return nil
}

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
		return
	}

	scanner := NewPositionalScanner(r, r.fileInput.MaxLogSize, r.Offset, r.splitter.SplitFunc)

	// Iterate over the tokenized file, emitting entries as we go
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := scanner.Scan()
		if !ok {
			if err := getScannerError(scanner); err != nil {
				r.Errorw("Failed during scan", zap.Error(err))
			}
			break
		}

		if err := r.emit(ctx, scanner.Bytes()); err != nil {
			r.Error("Failed to emit entry", zap.Error(err))
		}
		r.Offset = scanner.Pos()
	}
}

// Close will close the file
func (r *Reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Debugw("Problem closing reader", zap.Error(err))
		}
	}
}

// Emit creates an entry with the decoded message and sends it to the next
// operator in the pipeline
func (r *Reader) emit(ctx context.Context, msgBuf []byte) error {
	// Skip the entry if it's empty
	if len(msgBuf) == 0 {
		return nil
	}
	var e *entry.Entry
	var err error
	if r.fileInput.encoding.Encoding == encoding.Nop {
		e, err = r.fileInput.NewEntry(msgBuf)
		if err != nil {
			return fmt.Errorf("create entry: %s", err)
		}
	} else {
		msg, err := r.decode(msgBuf)
		if err != nil {
			return fmt.Errorf("decode: %s", err)
		}
		e, err = r.fileInput.NewEntry(msg)
		if err != nil {
			return fmt.Errorf("create entry: %s", err)
		}
	}

	if err := e.Set(r.fileInput.FilePathField, r.fileAttributes.Path); err != nil {
		return err
	}
	if err := e.Set(r.fileInput.FileNameField, r.fileAttributes.Name); err != nil {
		return err
	}

	if err := e.Set(r.fileInput.FilePathResolvedField, r.fileAttributes.ResolvedPath); err != nil {
		return err
	}

	if err := e.Set(r.fileInput.FileNameResolvedField, r.fileAttributes.ResolvedName); err != nil {
		return err
	}

	r.fileInput.Write(ctx, e)
	return nil
}

// decode converts the bytes in msgBuf to utf-8 from the configured encoding
func (r *Reader) decode(msgBuf []byte) (string, error) {
	for {
		r.decoder.Reset()
		nDst, _, err := r.decoder.Transform(r.decodeBuffer, msgBuf, true)
		if err != nil && err == transform.ErrShortDst {
			r.decodeBuffer = make([]byte, len(r.decodeBuffer)*2)
			continue
		} else if err != nil {
			return "", fmt.Errorf("transform encoding: %s", err)
		}
		return string(r.decodeBuffer[:nDst]), nil
	}
}

func getScannerError(scanner *PositionalScanner) error {
	err := scanner.Err()
	if err == bufio.ErrTooLong {
		return errors.NewError("log entry too large", "increase max_log_size or ensure that multiline regex patterns terminate")
	} else if err != nil {
		return errors.Wrap(err, "scanner error")
	}
	return nil
}

// Read from the file and update the fingerprint if necessary
func (r *Reader) Read(dst []byte) (int, error) {
	if len(r.Fingerprint.FirstBytes) == r.fileInput.fingerprintSize {
		return r.file.Read(dst)
	}
	n, err := r.file.Read(dst)
	appendCount := min0(n, r.fileInput.fingerprintSize-int(r.Offset))
	r.Fingerprint.FirstBytes = append(r.Fingerprint.FirstBytes[:r.Offset], dst[:appendCount]...)
	return n, err
}

func min0(a, b int) int {
	if a < 0 || b < 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
}
