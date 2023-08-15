// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decoder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"
)

type readerFactory struct {
	*zap.SugaredLogger
	readerConfig    *readerConfig
	fromBeginning   bool
	splitterFactory splitter.Factory
	encoding        encoding.Encoding
	headerConfig    *header.Config
}

func (f *readerFactory) newReader(file *os.File, fp *fingerprint.Fingerprint) (*reader, error) {
	lineSplitFunc, err := f.splitterFactory.Build(f.readerConfig.maxLogSize)
	if err != nil {
		return nil, err
	}
	return f.build(file, &readerMetadata{
		Fingerprint:    fp,
		FileAttributes: map[string]any{},
	}, lineSplitFunc)
}

// copy creates a deep copy of a reader
func (f *readerFactory) copy(old *reader, newFile *os.File) (*reader, error) {
	var err error
	lineSplitFunc := old.lineSplitFunc
	if lineSplitFunc == nil {
		lineSplitFunc, err = f.splitterFactory.Build(f.readerConfig.maxLogSize)
		if err != nil {
			return nil, err
		}
	}
	return f.build(newFile, &readerMetadata{
		Fingerprint:     old.Fingerprint.Copy(),
		Offset:          old.Offset,
		FileAttributes:  util.MapCopy(old.FileAttributes),
		HeaderFinalized: old.HeaderFinalized,
	}, lineSplitFunc)
}

func (f *readerFactory) newFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.readerConfig.fingerprintSize)
}

func (f *readerFactory) build(file *os.File, m *readerMetadata, lineSplitFunc bufio.SplitFunc) (r *reader, err error) {
	r = &reader{
		readerConfig:   f.readerConfig,
		readerMetadata: m,
		file:           file,
		SugaredLogger:  f.SugaredLogger.With("path", file.Name()),
		decoder:        decoder.New(f.encoding),
		lineSplitFunc:  lineSplitFunc,
	}

	if !f.fromBeginning {
		if err = r.offsetToEnd(); err != nil {
			return nil, err
		}
	}

	if f.headerConfig == nil || m.HeaderFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = f.readerConfig.emit
	} else {
		r.splitFunc = f.headerConfig.SplitFunc
		r.headerReader, err = header.NewReader(f.SugaredLogger, *f.headerConfig)
		if err != nil {
			return nil, err
		}
		r.processFunc = r.headerReader.Process
	}

	// Resolve file name and path attributes
	resolved := file.Name()

	// Dirty solution, waiting for this permanent fix https://githuf.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(file.Name())
		if err != nil {
			f.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		f.Errorf("resolve abs: %w", err)
	}

	if f.readerConfig.includeFileName {
		r.FileAttributes[logFileName] = filepath.Base(file.Name())
	} else if r.FileAttributes[logFileName] != nil {
		delete(r.FileAttributes, logFileName)
	}
	if f.readerConfig.includeFilePath {
		r.FileAttributes[logFilePath] = file.Name()
	} else if r.FileAttributes[logFilePath] != nil {
		delete(r.FileAttributes, logFilePath)
	}
	if f.readerConfig.includeFileNameResolved {
		r.FileAttributes[logFileNameResolved] = filepath.Base(abs)
	} else if r.FileAttributes[logFileNameResolved] != nil {
		delete(r.FileAttributes, logFileNameResolved)
	}
	if f.readerConfig.includeFilePathResolved {
		r.FileAttributes[logFilePathResolved] = abs
	} else if r.FileAttributes[logFilePathResolved] != nil {
		delete(r.FileAttributes, logFilePathResolved)
	}

	return r, nil
}
