// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	defaultMaxLogSize  = 1024 * 1024
	defaultFlushPeriod = 500 * time.Millisecond
)

func testFactory(t *testing.T, opts ...testFactoryOpt) (*Factory, *emittest.Sink) {
	cfg := &testFactoryCfg{
		fromBeginning:     true,
		fingerprintSize:   fingerprint.DefaultSize,
		initialBufferSize: scanner.DefaultBufferSize,
		maxLogSize:        defaultMaxLogSize,
		encoding:          unicode.UTF8,
		trimFunc:          trim.Whitespace,
		flushPeriod:       defaultFlushPeriod,
		sinkChanSize:      100,
		attributes: attrs.Resolver{
			IncludeFileName: true,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	splitFunc, err := cfg.splitCfg.Func(cfg.encoding, false, cfg.maxLogSize)
	require.NoError(t, err)

	sink := emittest.NewSink(emittest.WithCallBuffer(cfg.sinkChanSize))
	return &Factory{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		FromBeginning:     cfg.fromBeginning,
		FingerprintSize:   cfg.fingerprintSize,
		InitialBufferSize: cfg.initialBufferSize,
		MaxLogSize:        cfg.maxLogSize,
		Encoding:          cfg.encoding,
		SplitFunc:         splitFunc,
		TrimFunc:          cfg.trimFunc,
		FlushTimeout:      cfg.flushPeriod,
		EmitFunc:          sink.Callback,
		Attributes:        cfg.attributes,
	}, sink
}

type testFactoryOpt func(*testFactoryCfg)

type testFactoryCfg struct {
	fromBeginning     bool
	fingerprintSize   int
	initialBufferSize int
	maxLogSize        int
	encoding          encoding.Encoding
	splitCfg          split.Config
	trimFunc          trim.Func
	flushPeriod       time.Duration
	sinkChanSize      int
	attributes        attrs.Resolver
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

func withInitialBufferSize(size int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.initialBufferSize = size
	}
}

func withMaxLogSize(size int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.maxLogSize = size
	}
}

func withFlushPeriod(flushPeriod time.Duration) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.flushPeriod = flushPeriod
	}
}

func withSinkChanSize(n int) testFactoryOpt {
	return func(c *testFactoryCfg) {
		c.sinkChanSize = n
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

// TestNewReaderFromMetadataConcurrentAccess tests that FileAttributes map
// is not mutated in place, which would cause data races when the metadata
// is accessed concurrently (e.g., by test assertions or persistence).
func TestNewReaderFromMetadataConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	content := "test log line\n"
	_, err := temp.WriteString(content)
	require.NoError(t, err)

	f, _ := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	// Create initial reader
	r1, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	// Get metadata (simulating persistence or reader recycling)
	metadata := r1.Close()

	// Store original map reference to detect if it's mutated
	originalAttrs := metadata.FileAttributes
	originalLen := len(originalAttrs)

	// Create a done channel for synchronization
	done := make(chan bool)

	// Goroutine 1: Continuously read the FileAttributes map
	// (simulating test assertions or persistence reading)
	go func() {
		defer func() { done <- true }()
		for range 100 {
			// This simulates what reflect.DeepEqual does in test assertions
			_ = len(originalAttrs)
			for k, v := range originalAttrs {
				_ = k
				_ = v
			}
			time.Sleep(time.Microsecond)
		}
	}()

	// Goroutine 2: Create new reader from metadata
	// (simulating the polling goroutine calling NewReaderFromMetadata)
	go func() {
		defer func() { done <- true }()
		for range 100 {
			temp2 := filetest.OpenFile(t, temp.Name())
			_, err := f.NewReaderFromMetadata(temp2, metadata)
			if err != nil {
				t.Errorf("NewReaderFromMetadata failed: %v", err)
			}
			time.Sleep(time.Microsecond)
		}
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify that the original map was not mutated
	// (it should still have the same length, though a new map may have been created)
	require.Len(t, originalAttrs, originalLen,
		"Original FileAttributes map should not be mutated")
}
