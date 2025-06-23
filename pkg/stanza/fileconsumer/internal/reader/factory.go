// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
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
	Compression             string
	AcquireFSLock           bool
}

func (f *Factory) NewFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.NewFromFile(file, f.FingerprintSize, f.Compression != "")
}

func (f *Factory) NewReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	attributes, err := f.Attributes.Resolve(file)
	if err != nil {
		return nil, err
	}
	var filetype string
	if filepath.Ext(file.Name()) == gzipExtension {
		filetype = gzipExtension
	}

	m := &Metadata{
		Fingerprint:    fp,
		FileAttributes: attributes,
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
		shorter, rereadErr := fingerprint.NewFromFile(file, r.fingerprintSize, r.compression != "")
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
