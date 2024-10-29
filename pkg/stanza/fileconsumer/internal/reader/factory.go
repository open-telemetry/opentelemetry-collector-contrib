// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
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
	component.TelemetrySettings
	HeaderConfig            *header.Config
	FromBeginning           bool
	FingerprintSize         int
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
	Compression             string
	AcquireFSLock           bool
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.NewFromFile(file, f.FingerprintSize)
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	attributes, err := f.Attributes.Resolve(file)
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
		Metadata:             m,
		set:                  f.TelemetrySettings,
		file:                 file,
		fileName:             file.Name(),
		fingerprintSize:      f.FingerprintSize,
		initialBufferSize:    f.InitialBufferSize,
		maxLogSize:           f.MaxLogSize,
		decoder:              decode.New(f.Encoding),
		lineSplitFunc:        f.SplitFunc,
		deleteAtEOF:          f.DeleteAtEOF,
		includeFileRecordNum: f.IncludeFileRecordNumber,
		compression:          f.Compression,
		acquireFSLock:        f.AcquireFSLock,
	}
	r.set.Logger = r.set.Logger.With(zap.String("path", r.fileName))

	if r.Fingerprint.Len() > r.fingerprintSize {
		// User has reconfigured fingerprint_size
		shorter, rereadErr := fingerprint.NewFromFile(file, r.fingerprintSize)
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

	flushFunc := m.FlushState.Func(f.SplitFunc, f.FlushTimeout)
	r.lineSplitFunc = trim.WithFunc(trim.ToLength(flushFunc, f.MaxLogSize), f.TrimFunc)
	r.emitFunc = f.EmitFunc
	if f.HeaderConfig == nil || m.HeaderFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = r.emitFunc
	} else {
		r.headerReader, err = header.NewReader(f.TelemetrySettings, *f.HeaderConfig)
		if err != nil {
			return nil, err
		}
		r.splitFunc = f.HeaderConfig.SplitFunc
		r.processFunc = r.headerReader.Process
	}

	attributes, err := f.Attributes.Resolve(file)
	if err != nil {
		return nil, err
	}
	// Copy attributes into existing map to avoid overwriting header attributes
	for k, v := range attributes {
		r.FileAttributes[k] = v
	}

	return r, nil
}
