// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type readerFactory struct {
	*zap.SugaredLogger
	readerConfig    *readerConfig
	fromBeginning   bool
	splitterFactory splitterFactory
	encodingConfig  helper.EncodingConfig
	headerSettings  *headerSettings
}

func (f *readerFactory) newReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	return f.newReaderBuilder().
		withFile(file).
		withFingerprint(fp).
		build()
}

// copy creates a deep copy of a Reader
func (f *readerFactory) copy(old *Reader, newFile *os.File) (*Reader, error) {
	return f.newReaderBuilder().
		withFile(newFile).
		withFingerprint(old.Fingerprint.Copy()).
		withOffset(old.Offset).
		withSplitterFunc(old.lineSplitFunc).
		withHeaderAttributes(mapCopy(old.FileAttributes.HeaderAttributes)).
		withHeaderFinalized(old.HeaderFinalized).
		build()
}

func (f *readerFactory) unsafeReader() (*Reader, error) {
	return f.newReaderBuilder().build()
}

func (f *readerFactory) newFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.readerConfig.fingerprintSize)
}

type readerBuilder struct {
	*readerFactory
	file             *os.File
	fp               *fingerprint.Fingerprint
	offset           int64
	splitFunc        bufio.SplitFunc
	headerFinalized  bool
	headerAttributes map[string]any
}

func (f *readerFactory) newReaderBuilder() *readerBuilder {
	return &readerBuilder{readerFactory: f, headerAttributes: map[string]any{}}
}

func (b *readerBuilder) withSplitterFunc(s bufio.SplitFunc) *readerBuilder {
	b.splitFunc = s
	return b
}

func (b *readerBuilder) withFile(f *os.File) *readerBuilder {
	b.file = f
	return b
}

func (b *readerBuilder) withFingerprint(fp *fingerprint.Fingerprint) *readerBuilder {
	b.fp = fp
	return b
}

func (b *readerBuilder) withOffset(offset int64) *readerBuilder {
	b.offset = offset
	return b
}

func (b *readerBuilder) withHeaderFinalized(finalized bool) *readerBuilder {
	b.headerFinalized = finalized
	return b
}

func (b *readerBuilder) withHeaderAttributes(attrs map[string]any) *readerBuilder {
	b.headerAttributes = attrs
	return b
}

func (b *readerBuilder) build() (r *Reader, err error) {
	r = &Reader{
		readerConfig:    b.readerConfig,
		Offset:          b.offset,
		headerSettings:  b.headerSettings,
		HeaderFinalized: b.headerFinalized,
	}

	if b.splitFunc != nil {
		r.lineSplitFunc = b.splitFunc
	} else {
		r.lineSplitFunc, err = b.splitterFactory.Build(b.readerConfig.maxLogSize)
		if err != nil {
			return
		}
	}

	enc, err := b.encodingConfig.Build()
	if err != nil {
		return
	}
	r.encoding = enc

	if b.file != nil {
		r.file = b.file
		r.SugaredLogger = b.SugaredLogger.With("path", b.file.Name())
		r.FileAttributes, err = resolveFileAttributes(b.file.Name())
		if err != nil {
			b.Errorf("resolve attributes: %w", err)
		}

		// unsafeReader has the file set to nil, so don't try emending its offset.
		if !b.fromBeginning {
			if err = r.offsetToEnd(); err != nil {
				return nil, err
			}
		}
	} else {
		r.SugaredLogger = b.SugaredLogger.With("path", "uninitialized")
		r.FileAttributes = &FileAttributes{}
	}

	if b.fp != nil {
		r.Fingerprint = b.fp
	} else if b.file != nil {
		r.Fingerprint, err = b.readerFactory.newFingerprint(r.file)
		if err != nil {
			return nil, err
		}
	}

	r.FileAttributes.HeaderAttributes = b.headerAttributes

	if b.headerSettings == nil || b.headerFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = b.readerConfig.emit
		return r, nil
	}

	// We are reading the header so we should start with the header split func
	r.splitFunc = b.headerSettings.splitFunc

	outOp := newHeaderPipelineOutput(b.SugaredLogger)
	p, err := pipeline.Config{
		Operators:     b.headerSettings.config.MetadataOperators,
		DefaultOutput: outOp,
	}.Build(b.SugaredLogger)

	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	if err := p.Start(storage.NewNopClient()); err != nil {
		return nil, fmt.Errorf("failed to start header pipeline: %w", err)
	}

	r.headerPipeline = p
	r.headerPipelineOutput = outOp

	// Set initial emit func to header function
	r.processFunc = r.consumeHeaderLine

	return r, nil
}
