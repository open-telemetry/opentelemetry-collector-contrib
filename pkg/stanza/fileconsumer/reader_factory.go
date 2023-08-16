// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type readerFactory struct {
	*zap.SugaredLogger
	readerConfig    *readerConfig
	fromBeginning   bool
	splitterFactory splitter.Factory
	encodingConfig  helper.EncodingConfig
	headerConfig    *header.Config
}

func (f *readerFactory) newReader(file *os.File, fp *fingerprint.Fingerprint) (*reader, error) {
	return readerBuilder{
		readerFactory: f,
		file:          file,
		readerMetadata: &readerMetadata{
			Fingerprint:    fp,
			FileAttributes: map[string]any{},
		},
	}.build()
}

func (f *readerFactory) newFromMetadata(m *readerMetadata) (*reader, error) {
	return readerBuilder{
		readerFactory:  f,
		readerMetadata: m,
	}.build()
}

// copy creates a deep copy of a reader
func (f *readerFactory) copy(old *reader, newFile *os.File) (*reader, error) {
	return readerBuilder{
		readerFactory: f,
		file:          newFile,
		splitFunc:     old.lineSplitFunc,
		readerMetadata: &readerMetadata{
			Fingerprint:     old.Fingerprint.Copy(),
			Offset:          old.Offset,
			FileAttributes:  util.MapCopy(old.FileAttributes),
			HeaderFinalized: old.HeaderFinalized,
		},
	}.build()
}

func (f *readerFactory) newFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.readerConfig.fingerprintSize)
}

type readerBuilder struct {
	*readerFactory
	file           *os.File
	readerMetadata *readerMetadata
	splitFunc      bufio.SplitFunc
}

func (b readerBuilder) build() (r *reader, err error) {
	r = &reader{
		readerConfig:   b.readerConfig,
		readerMetadata: b.readerMetadata,
	}

	if b.splitFunc != nil {
		r.lineSplitFunc = b.splitFunc
	} else {
		r.lineSplitFunc, err = b.splitterFactory.Build(b.readerConfig.maxLogSize)
		if err != nil {
			return nil, err
		}
	}

	encoding, err := helper.LookupEncoding(b.encodingConfig.Encoding)
	if err != nil {
		return nil, err
	}
	r.decoder = helper.NewDecoder(encoding)

	if b.headerConfig == nil || b.readerMetadata.HeaderFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = b.readerConfig.emit
	} else {
		r.splitFunc = b.headerConfig.SplitFunc
		r.headerReader, err = header.NewReader(b.SugaredLogger, *b.headerConfig)
		if err != nil {
			return nil, err
		}
		r.processFunc = r.headerReader.Process
	}

	if b.file == nil {
		return r, nil
	}

	r.file = b.file
	r.SugaredLogger = b.SugaredLogger.With("path", b.file.Name())

	// Resolve file name and path attributes
	resolved := b.file.Name()

	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(b.file.Name())
		if err != nil {
			b.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		b.Errorf("resolve abs: %w", err)
	}

	if b.readerConfig.includeFileName {
		r.FileAttributes[logFileName] = filepath.Base(b.file.Name())
	} else if r.FileAttributes[logFileName] != nil {
		delete(r.FileAttributes, logFileName)
	}
	if b.readerConfig.includeFilePath {
		r.FileAttributes[logFilePath] = b.file.Name()
	} else if r.FileAttributes[logFilePath] != nil {
		delete(r.FileAttributes, logFilePath)
	}
	if b.readerConfig.includeFileNameResolved {
		r.FileAttributes[logFileNameResolved] = filepath.Base(abs)
	} else if r.FileAttributes[logFileNameResolved] != nil {
		delete(r.FileAttributes, logFileNameResolved)
	}
	if b.readerConfig.includeFilePathResolved {
		r.FileAttributes[logFilePathResolved] = abs
	} else if r.FileAttributes[logFilePathResolved] != nil {
		delete(r.FileAttributes, logFilePathResolved)
	}

	if !b.fromBeginning {
		if err = r.offsetToEnd(); err != nil {
			return nil, err
		}
	}

	return r, nil
}
