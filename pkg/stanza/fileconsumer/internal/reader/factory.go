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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type Factory struct {
	*zap.SugaredLogger
	Config        *Config
	FromBeginning bool
	Encoding      encoding.Encoding
	HeaderConfig  *header.Config
	SplitFunc     bufio.SplitFunc
	TrimFunc      trim.Func
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.Config.FingerprintSize)
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	m := &Metadata{Fingerprint: fp, FileAttributes: map[string]any{}}
	if f.Config.FlushTimeout > 0 {
		m.FlushState = &flush.State{LastDataChange: time.Now()}
	}
	return f.NewReaderFromMetadata(file, m)
}

func (f *Factory) NewReaderFromMetadata(file *os.File, m *Metadata) (r *Reader, err error) {
	r = &Reader{
		Config:        f.Config,
		Metadata:      m,
		file:          file,
		fileName:      file.Name(),
		logger:        f.SugaredLogger.With("path", file.Name()),
		decoder:       decode.New(f.Encoding),
		lineSplitFunc: f.SplitFunc,
	}

	flushFunc := m.FlushState.Func(f.SplitFunc, f.Config.FlushTimeout)
	r.lineSplitFunc = trim.WithFunc(trim.ToLength(flushFunc, f.Config.MaxLogSize), f.TrimFunc)

	if !f.FromBeginning {
		if err = r.offsetToEnd(); err != nil {
			return nil, err
		}
	}

	if f.HeaderConfig == nil || m.HeaderFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = f.Config.Emit
	} else {
		r.splitFunc = f.HeaderConfig.SplitFunc
		r.headerReader, err = header.NewReader(f.SugaredLogger, *f.HeaderConfig)
		if err != nil {
			return nil, err
		}
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

	if f.Config.IncludeFileName {
		r.FileAttributes[attrs.LogFileName] = filepath.Base(r.fileName)
	} else if r.FileAttributes[attrs.LogFileName] != nil {
		delete(r.FileAttributes, attrs.LogFileName)
	}
	if f.Config.IncludeFilePath {
		r.FileAttributes[attrs.LogFilePath] = r.fileName
	} else if r.FileAttributes[attrs.LogFilePath] != nil {
		delete(r.FileAttributes, attrs.LogFilePath)
	}
	if f.Config.IncludeFileNameResolved {
		r.FileAttributes[attrs.LogFileNameResolved] = filepath.Base(abs)
	} else if r.FileAttributes[attrs.LogFileNameResolved] != nil {
		delete(r.FileAttributes, attrs.LogFileNameResolved)
	}
	if f.Config.IncludeFilePathResolved {
		r.FileAttributes[attrs.LogFilePathResolved] = abs
	} else if r.FileAttributes[attrs.LogFilePathResolved] != nil {
		delete(r.FileAttributes, attrs.LogFilePathResolved)
	}

	return r, nil
}
