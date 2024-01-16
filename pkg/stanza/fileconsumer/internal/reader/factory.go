// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
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
	HeaderConfig    *header.Config
	FromBeginning   bool
	FingerprintSize int
	MaxLogSize      int
	Encoding        encoding.Encoding
	SplitFunc       bufio.SplitFunc
	TrimFunc        trim.Func
	FlushTimeout    time.Duration
	EmitFunc        emit.Callback
	Attributes      attrs.Resolver
	DeleteAtEOF     bool
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.FingerprintSize)
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	attributes, err := f.Attributes.Resolve(file.Name())
	if err != nil {
		return nil, err
	}
	m := &Metadata{Fingerprint: fp, FileAttributes: attributes}
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
		var info os.FileInfo
		if info, err = r.file.Stat(); err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}
		r.Offset = info.Size()
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

	attributes, err := f.Attributes.Resolve(file.Name())
	if err != nil {
		return nil, err
	}
	// Copy attributes into existing map to avoid overwriting header attributes
	for k, v := range attributes {
		r.FileAttributes[k] = v
	}
	return r, nil
}
