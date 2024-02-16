// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
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
		attributes: attrs.Resolver{
			IncludeFileName: true,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	splitFunc, err := cfg.splitCfg.Func(cfg.encoding, false, cfg.maxLogSize)
	require.NoError(t, err)

	sink := emittest.NewSink(emittest.WithCallBuffer(cfg.sinkCallBufferSize))
	return &Factory{
		SugaredLogger:   testutil.Logger(t),
		FromBeginning:   cfg.fromBeginning,
		FingerprintSize: cfg.fingerprintSize,
		MaxLogSize:      cfg.maxLogSize,
		Encoding:        cfg.encoding,
		SplitFunc:       splitFunc,
		TrimFunc:        cfg.trimFunc,
		FlushTimeout:    cfg.flushPeriod,
		EmitFunc:        sink.Callback,
		Attributes:      cfg.attributes,
	}, sink
}

type testFactoryOpt func(*testFactoryCfg)

type testFactoryCfg struct {
	fromBeginning      bool
	fingerprintSize    int
	maxLogSize         int
	encoding           encoding.Encoding
	splitCfg           split.Config
	trimFunc           trim.Func
	flushPeriod        time.Duration
	sinkCallBufferSize int
	attributes         attrs.Resolver
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

func fromEnd() testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.fromBeginning = false
	}
}

func TestStartAt(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	content := "some text\n"
	_, err := temp.WriteString(content)
	require.NoError(t, err)

	f, _ := testFactory(t, withFingerprintSize(len(content)*2))
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	f, _ = testFactory(t, withFingerprintSize(len(content)/2), fromEnd())
	fp, err = f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err = f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), r.Offset)

	f, _ = testFactory(t, withFingerprintSize(len(content)/2))
	fp, err = f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err = f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	f, _ = testFactory(t, withFingerprintSize(len(content)/2), fromEnd())
	fp, err = f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err = f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), r.Offset)
}
