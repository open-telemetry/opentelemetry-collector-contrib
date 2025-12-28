// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/compression"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenlen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	DefaultMaxLogSize   = 1024 * 1024
	DefaultFlushPeriod  = 500 * time.Millisecond
	DefaultMaxBatchSize = 100
)

type Factory struct {
	component.TelemetrySettings
	HeaderConfig            *header.Config
	FromBeginning           bool
	FingerprintSize         int
	BufPool                 sync.Pool
	InitialBufferSize       int
	MaxLogSize              int
	Encoding                encoding.Encoding
	SplitFunc               bufio.SplitFunc
	TrimFunc                trim.Func
	FlushTimeout            time.Duration
	EmitFunc                emit.Callback
	Attributes              attrs.Resolver
	DeleteAtEOF             bool
	IncludeFileRecordNumber bool
	IncludeFileRecordOffset bool
	Compression             string
	AcquireFSLock           bool
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.NewFromFile(file, f.FingerprintSize, f.Compression != "", f.Logger)
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	attributes, err := f.Attributes.Resolve(file)
	if err != nil {
		return nil, err
	}
	var filetype string

	if f.Compression != "" && compression.IsGzipFile(file, f.Logger) {
		filetype = gzipExtension
	}

	m := &Metadata{
		Fingerprint:    fp,
		FileAttributes: attributes,
		OriginalPath:   file.Name(),
		TokenLenState:  tokenlen.State{},
		FlushState: flush.State{
			LastDataChange: time.Now(),
		},
		FileType: filetype,
	}
	return f.NewReaderFromMetadata(file, m)
}

func (f *Factory) NewReaderFromMetadata(file *os.File, m *Metadata) (r *Reader, err error) {
	r = &Reader{
		Metadata:          m,
		set:               f.TelemetrySettings,
		file:              file,
		fileName:          file.Name(),
		fingerprintSize:   f.FingerprintSize,
		bufPool:           &f.BufPool,
		initialBufferSize: f.InitialBufferSize,
		maxLogSize:        f.MaxLogSize,
		decoder:           f.Encoding.NewDecoder(),
		deleteAtEOF:       f.DeleteAtEOF,
		compression:       f.Compression,
		acquireFSLock:     f.AcquireFSLock,
		maxBatchSize:      DefaultMaxBatchSize,
		emitFunc:          f.EmitFunc,
	}
	r.set.Logger = r.set.Logger.With(zap.String("path", r.fileName))

	if r.Fingerprint.Len() > r.fingerprintSize {
		// User has reconfigured fingerprint_size
		shorter, rereadErr := fingerprint.NewFromFile(file, r.fingerprintSize, r.compression != "", r.set.Logger)
		if rereadErr != nil {
			return nil, fmt.Errorf("reread fingerprint: %w", rereadErr)
		}
		if !r.Fingerprint.StartsWith(shorter) {
			return nil, errors.New("file truncated")
		}
		m.Fingerprint = shorter
	}

	if !f.FromBeginning {
		var info os.FileInfo
		if info, err = r.file.Stat(); err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}
		r.Offset = info.Size()
	}

	tokenLenFunc := m.TokenLenState.Func(f.SplitFunc)
	flushFunc := m.FlushState.Func(tokenLenFunc, f.FlushTimeout)
	r.contentSplitFunc = trim.WithFunc(trim.ToLength(flushFunc, f.MaxLogSize), f.TrimFunc)

	if f.HeaderConfig != nil && !m.HeaderFinalized {
		r.headerSplitFunc = f.HeaderConfig.SplitFunc
		r.headerReader, err = header.NewReader(f.TelemetrySettings, *f.HeaderConfig)
		if err != nil {
			return nil, err
		}
	}

	// Preserve original path if metadata is being reused (rotation scenario)
	// Reset OriginalPath if the file name changed in a way that suggests it's not a rotation
	if m.OriginalPath == "" {
		// First time this metadata is used, capture the original path
		m.OriginalPath = r.fileName
	} else if !isRotation(m.OriginalPath, r.fileName) {
		// If the file name changed in a way that doesn't look like rotation,
		// this is likely a different file recovered from archive, reset the original path
		m.OriginalPath = r.fileName
	}

	attributes, err := f.Attributes.Resolve(file)
	if err != nil {
		return nil, err
	}

	// Override file path attributes with original path to preserve them through rotation
	if f.Attributes.IncludeFilePath {
		attributes[attrs.LogFilePath] = m.OriginalPath
	}
	if f.Attributes.IncludeFileName {
		attributes[attrs.LogFileName] = filepath.Base(m.OriginalPath)
	}

	// Copy attributes into existing map to avoid overwriting header attributes
	maps.Copy(r.FileAttributes, attributes)

	return r, nil
}

// isRotation checks if newPath appears to be a rotation of originalPath.
// Rotation typically means the new filename starts with the original filename
// and adds a suffix (like .1, .20250219-233547, etc.)
// Returns true if it looks like a rotation, false otherwise.
func isRotation(originalPath, newPath string) bool {
	// If paths are identical, definitely not a rotation
	if originalPath == newPath {
		return true
	}

	// Directory must be the same for rotation
	if filepath.Dir(originalPath) != filepath.Dir(newPath) {
		return false
	}

	originalBase := filepath.Base(originalPath)
	newBase := filepath.Base(newPath)

	// Check if new filename starts with original filename (rotation adds suffix)
	// e.g., app.log -> app.log.1 or app.log -> app.log.20250219-233547
	return len(newBase) > len(originalBase) && strings.HasPrefix(newBase, originalBase)
}
