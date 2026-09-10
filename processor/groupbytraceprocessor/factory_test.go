// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

func TestDefaultConfiguration(t *testing.T) {
	// test
	c := createDefaultConfig().(*Config)

	// verify
	assert.Equal(t, defaultNumTraces, c.NumTraces)
	assert.Equal(t, defaultNumWorkers, c.NumWorkers)
	assert.Equal(t, defaultWaitDuration, c.WaitDuration)
	assert.Equal(t, defaultDiscardOrphans, c.DiscardOrphans)
	assert.Equal(t, defaultStoreOnDisk, c.StoreOnDisk)
}

func TestCreateTestProcessor(t *testing.T) {
	c := createDefaultConfig().(*Config)

	// test
	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), c, consumertest.NewNop())

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestCreateTestProcessorWithNotImplementedOptions(t *testing.T) {
	// prepare
	f := NewFactory()

	// test
	for _, tt := range []struct {
		config      *Config
		expectedErr error
	}{
		{
			&Config{
				DiscardOrphans: true,
			},
			errDiscardOrphansNotSupported,
		},
		{
			&Config{
				StoreOnDisk: true,
			},
			errDiskStorageNotSupported,
		},
	} {
		p, err := f.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), tt.config, consumertest.NewNop())

		// verify
		assert.ErrorIs(t, tt.expectedErr, err)
		assert.Nil(t, p)
	}
}

// TestCreateProcessorServiceEmitNumTracesLessThanNumWorkers verifies that
// num_traces < num_workers (integer division → 0-size ring buffer) does not
// panic when the processor is created or used.
func TestCreateProcessorServiceEmitNumTracesLessThanNumWorkers(t *testing.T) {
	cfg := &Config{
		NumTraces:    1,
		NumWorkers:   2,
		WaitDuration: time.Second,
		EmitStrategy: EmitStrategyService,
	}
	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
}

// Traces are routed to a worker by trace ID, so each worker keeps its own span
// storage. Sharing one would put every worker behind a single lock for every
// span buffered.
func TestCreateProcessorServiceEmitGivesEachWorkerItsOwnStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NumTraces = 100
	cfg.NumWorkers = 4
	cfg.EmitStrategy = EmitStrategyService

	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	workers := p.(*groupByTraceProcessor).eventMachine.workers
	require.Len(t, workers, cfg.NumWorkers)

	seen := map[subtraceStorage]bool{}
	for i, w := range workers {
		require.NotNil(t, w.subSt, "worker %d has no storage", i)
		assert.False(t, seen[w.subSt], "worker %d shares its storage with another worker", i)
		seen[w.subSt] = true
	}
}

// The trace strategy has no per-worker span storage to allocate.
func TestCreateProcessorTraceEmitHasNoSubtraceStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NumWorkers = 2

	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	for i, w := range p.(*groupByTraceProcessor).eventMachine.workers {
		assert.Nil(t, w.subSt, "worker %d allocated subtrace storage in trace mode", i)
	}
}
