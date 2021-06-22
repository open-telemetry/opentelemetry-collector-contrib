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

package file

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"github.com/open-telemetry/opentelemetry-log-collection/errors"
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

	*zap.SugaredLogger `json:"-"`
}

// NewReader creates a new file reader
func (f *InputOperator) NewReader(path string, file *os.File, fp *Fingerprint) (*Reader, error) {
	r := &Reader{
		Fingerprint:    fp,
		file:           file,
		fileInput:      f,
		SugaredLogger:  f.SugaredLogger.With("path", path),
		decoder:        f.encoding.Encoding.NewDecoder(),
		decodeBuffer:   make([]byte, 1<<12),
		fileAttributes: f.resolveFileAttributes(path),
	}
	return r, nil
}

// Copy creates a deep copy of a Reader
func (f *Reader) Copy(file *os.File) (*Reader, error) {
	reader, err := f.fileInput.NewReader(f.fileAttributes.Path, file, f.Fingerprint.Copy())
	if err != nil {
		return nil, err
	}
	reader.Offset = f.Offset
	return reader, nil
}

// InitializeOffset sets the starting offset
func (f *Reader) InitializeOffset(startAtBeginning bool) error {
	if !startAtBeginning {
		info, err := f.file.Stat()
		if err != nil {
			return fmt.Errorf("stat: %s", err)
		}
		f.Offset = info.Size()
	}

	return nil
}

// ReadToEnd will read until the end of the file
func (f *Reader) ReadToEnd(ctx context.Context) {
	if _, err := f.file.Seek(f.Offset, 0); err != nil {
		f.Errorw("Failed to seek", zap.Error(err))
		return
	}

	fr := NewFingerprintUpdatingReader(f.file, f.Offset, f.Fingerprint, f.fileInput.fingerprintSize)
	scanner := NewPositionalScanner(fr, f.fileInput.MaxLogSize, f.Offset, f.fileInput.SplitFunc)

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
				f.Errorw("Failed during scan", zap.Error(err))
			}
			break
		}

		if err := f.emit(ctx, scanner.Bytes()); err != nil {
			f.Error("Failed to emit entry", zap.Error(err))
		}
		f.Offset = scanner.Pos()
	}
}

// Close will close the file
func (f *Reader) Close() {
	if f.file != nil {
		if err := f.file.Close(); err != nil {
			f.Debugf("Problem closing reader", "Error", err.Error())
		}
	}
}

// Emit creates an entry with the decoded message and sends it to the next
// operator in the pipeline
func (f *Reader) emit(ctx context.Context, msgBuf []byte) error {
	// Skip the entry if it's empty
	if len(msgBuf) == 0 {
		return nil
	}

	msg, err := f.decode(msgBuf)
	if err != nil {
		return fmt.Errorf("decode: %s", err)
	}

	e, err := f.fileInput.NewEntry(msg)
	if err != nil {
		return fmt.Errorf("create entry: %s", err)
	}

	if err := e.Set(f.fileInput.FilePathField, f.fileAttributes.Path); err != nil {
		return err
	}
	if err := e.Set(f.fileInput.FileNameField, f.fileAttributes.Name); err != nil {
		return err
	}

	if err := e.Set(f.fileInput.FilePathResolvedField, f.fileAttributes.ResolvedPath); err != nil {
		return err
	}

	if err := e.Set(f.fileInput.FileNameResolvedField, f.fileAttributes.ResolvedName); err != nil {
		return err
	}

	f.fileInput.Write(ctx, e)
	return nil
}

// decode converts the bytes in msgBuf to utf-8 from the configured encoding
func (f *Reader) decode(msgBuf []byte) (string, error) {
	for {
		f.decoder.Reset()
		nDst, _, err := f.decoder.Transform(f.decodeBuffer, msgBuf, true)
		if err != nil && err == transform.ErrShortDst {
			f.decodeBuffer = make([]byte, len(f.decodeBuffer)*2)
			continue
		} else if err != nil {
			return "", fmt.Errorf("transform encoding: %s", err)
		}
		return string(f.decodeBuffer[:nDst]), nil
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

// NewFingerprintUpdatingReader creates a new FingerprintUpdatingReader starting starting at the given offset
func NewFingerprintUpdatingReader(r io.Reader, offset int64, f *Fingerprint, fingerprintSize int) *FingerprintUpdatingReader {
	return &FingerprintUpdatingReader{
		fingerprint:     f,
		fingerprintSize: fingerprintSize,
		reader:          r,
		offset:          offset,
	}
}

// FingerprintUpdatingReader wraps another reader, and updates the fingerprint
// with each read in the first fingerPrintSize bytes
type FingerprintUpdatingReader struct {
	fingerprint     *Fingerprint
	fingerprintSize int
	reader          io.Reader
	offset          int64
}

// Read reads from the wrapped reader, saving the read bytes to the fingerprint
func (f *FingerprintUpdatingReader) Read(dst []byte) (int, error) {
	if len(f.fingerprint.FirstBytes) == f.fingerprintSize {
		return f.reader.Read(dst)
	}
	n, err := f.reader.Read(dst)
	appendCount := min0(n, f.fingerprintSize-int(f.offset))
	f.fingerprint.FirstBytes = append(f.fingerprint.FirstBytes[:f.offset], dst[:appendCount]...)
	f.offset += int64(n)
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
