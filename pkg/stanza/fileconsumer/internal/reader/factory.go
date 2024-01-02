// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	DefaultMaxLogSize  = 1024 * 1024
	DefaultFlushPeriod = 500 * time.Millisecond
)

type Factory struct {
	*zap.SugaredLogger
	HeaderConfig            *header.Config
	FromBeginning           bool
	FingerprintSize         int
	MaxLogSize              int
	Encoding                encoding.Encoding
	SplitFunc               bufio.SplitFunc
	TrimFunc                trim.Func
	FlushTimeout            time.Duration
	EmitFunc                emit.Callback
	IncludeFileName         bool
	IncludeFilePath         bool
	IncludeFileNameResolved bool
	IncludeFilePathResolved bool
	DeleteAtEOF             bool
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.FingerprintSize)
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	m := &Metadata{Fingerprint: fp, FileAttributes: map[string]any{}}
	if f.FlushTimeout > 0 {
		m.FlushState = &flush.State{LastDataChange: time.Now()}
	}
	return f.NewReaderFromMetadata(file, m)
}

func (f *Factory) NewReaderFromMetadata(file *os.File, m *Metadata) (r *Reader, err error) {
	r = &Reader{
		Metadata:        m,
		logger:          f.SugaredLogger.With("path", file.Name()),
		file:            file,
		fileName:        file.Name(),
		fingerprintSize: f.FingerprintSize,
		maxLogSize:      f.MaxLogSize,
		decoder:         decode.New(f.Encoding),
		lineSplitFunc:   f.SplitFunc,
		deleteAtEOF:     f.DeleteAtEOF,
	}

	flushFunc := m.FlushState.Func(f.SplitFunc, f.FlushTimeout)
	r.lineSplitFunc = trim.WithFunc(trim.ToLength(flushFunc, f.MaxLogSize), f.TrimFunc)

	if !f.FromBeginning {
		if err = r.offsetToEnd(); err != nil {
			return nil, err
		}
	}

	r.emitFunc = f.EmitFunc
	if f.HeaderConfig == nil || m.HeaderFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = r.emitFunc
	} else {
		r.headerReader, err = header.NewReader(f.SugaredLogger, *f.HeaderConfig)
		if err != nil {
			return nil, err
		}
		r.splitFunc = f.HeaderConfig.SplitFunc
		r.processFunc = r.headerReader.Process
	}

	// Resolve file name and path attributes
	resolved := r.fileName

	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(r.fileName)
		if err != nil {
			f.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		f.Errorf("resolve abs: %w", err)
	}

	if f.IncludeFileName {
		r.FileAttributes[attrs.LogFileName] = filepath.Base(r.fileName)
	} else if r.FileAttributes[attrs.LogFileName] != nil {
		delete(r.FileAttributes, attrs.LogFileName)
	}
	if f.IncludeFilePath {
		r.FileAttributes[attrs.LogFilePath] = r.fileName
	} else if r.FileAttributes[attrs.LogFilePath] != nil {
		delete(r.FileAttributes, attrs.LogFilePath)
	}
	if f.IncludeFileNameResolved {
		r.FileAttributes[attrs.LogFileNameResolved] = filepath.Base(abs)
	} else if r.FileAttributes[attrs.LogFileNameResolved] != nil {
		delete(r.FileAttributes, attrs.LogFileNameResolved)
	}
	if f.IncludeFilePathResolved {
		r.FileAttributes[attrs.LogFilePathResolved] = abs
	} else if r.FileAttributes[attrs.LogFilePathResolved] != nil {
		delete(r.FileAttributes, attrs.LogFilePathResolved)
	}

	return r, nil
}
