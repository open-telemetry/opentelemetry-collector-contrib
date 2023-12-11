// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	defaultMaxLogSize  = 1024 * 1024
	defaultFlushPeriod = 500 * time.Millisecond
)

func testFactory(t *testing.T, opts ...testFactoryOpt) (*Factory, *emittest.Sink) {
	cfg := &testFactoryCfg{
		fromBeginning:      true,
		fingerprintSize:    fingerprint.DefaultSize,
		maxLogSize:         defaultMaxLogSize,
		encoding:           unicode.UTF8,
		trimFunc:           trim.Whitespace,
		flushPeriod:        defaultFlushPeriod,
		sinkCallBufferSize: 100,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	splitFunc, err := cfg.splitCfg.Func(cfg.encoding, false, cfg.maxLogSize)
	require.NoError(t, err)

	sink := emittest.NewSink(emittest.WithCallBuffer(cfg.sinkCallBufferSize))
	return &Factory{
		SugaredLogger:           testutil.Logger(t),
		FromBeginning:           cfg.fromBeginning,
		FingerprintSize:         cfg.fingerprintSize,
		MaxLogSize:              cfg.maxLogSize,
		Encoding:                cfg.encoding,
		SplitFunc:               splitFunc,
		TrimFunc:                cfg.trimFunc,
		FlushTimeout:            cfg.flushPeriod,
		EmitFunc:                sink.Callback,
		IncludeFileName:         cfg.includeFileName,
		IncludeFilePath:         cfg.includeFilePath,
		IncludeFileNameResolved: cfg.includeFileNameResolved,
		IncludeFilePathResolved: cfg.includeFilePathResolved,
	}, sink
}

type testFactoryOpt func(*testFactoryCfg)

type testFactoryCfg struct {
	fromBeginning           bool
	fingerprintSize         int
	maxLogSize              int
	encoding                encoding.Encoding
	splitCfg                split.Config
	trimFunc                trim.Func
	flushPeriod             time.Duration
	sinkCallBufferSize      int
	includeFileName         bool
	includeFilePath         bool
	includeFileNameResolved bool
	includeFilePathResolved bool
}

func withFingerprintSize(size int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.fingerprintSize = size
	}
}

func withSplitConfig(cfg split.Config) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.splitCfg = cfg
	}
}

func withMaxLogSize(maxLogSize int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.maxLogSize = maxLogSize
	}
}

func withFlushPeriod(flushPeriod time.Duration) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.flushPeriod = flushPeriod
	}
}

func withSinkBufferSize(n int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.sinkCallBufferSize = n
	}
}

func includeFileName() testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.includeFileName = true
	}
}

func includeFilePath() testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.includeFilePath = true
	}
}

func includeFileNameResolved() testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.includeFileNameResolved = true
	}
}

func includeFilePathResolved() testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.includeFilePathResolved = true
	}
}
