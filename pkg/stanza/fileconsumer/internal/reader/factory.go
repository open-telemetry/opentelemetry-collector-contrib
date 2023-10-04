// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"
)

type Factory struct {
	*zap.SugaredLogger
	Config          *Config
	FromBeginning   bool
	SplitterFactory splitter.Factory
	Encoding        encoding.Encoding
	HeaderConfig    *header.Config
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	return f.build(file, &Metadata{
		Fingerprint:    fp,
		FileAttributes: map[string]any{},
	}, f.SplitterFactory.SplitFunc())
}

// copy creates a deep copy of a reader
func (f *Factory) Copy(old *Reader, newFile *os.File) (*Reader, error) {
	lineSplitFunc := old.lineSplitFunc
	if lineSplitFunc == nil {
		lineSplitFunc = f.SplitterFactory.SplitFunc()
	}
	return f.build(newFile, &Metadata{
		Fingerprint:     old.Fingerprint.Copy(),
		Offset:          old.Offset,
		FileAttributes:  util.MapCopy(old.FileAttributes),
		HeaderFinalized: old.HeaderFinalized,
	}, lineSplitFunc)
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.Config.FingerprintSize)
}

func (f *Factory) build(file *os.File, m *Metadata, lineSplitFunc bufio.SplitFunc) (r *Reader, err error) {
	r = &Reader{
		Config:        f.Config,
		Metadata:      m,
		file:          file,
		FileName:      file.Name(),
		SugaredLogger: f.SugaredLogger.With("path", file.Name()),
		decoder:       decode.New(f.Encoding),
		lineSplitFunc: lineSplitFunc,
	}

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
	resolved := r.FileName

	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(r.FileName)
		if err != nil {
			f.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		f.Errorf("resolve abs: %w", err)
	}

	if f.Config.IncludeFileName {
		r.FileAttributes[attrs.LogFileName] = filepath.Base(r.FileName)
	} else if r.FileAttributes[attrs.LogFileName] != nil {
		delete(r.FileAttributes, attrs.LogFileName)
	}
	if f.Config.IncludeFilePath {
		r.FileAttributes[attrs.LogFilePath] = r.FileName
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
