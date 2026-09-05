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

func TestUnknownEmitStrategy(t *testing.T) {
	_, err := NewFactory().CreateTraces(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		&Config{
			NumTraces:    10,
			NumWorkers:   1,
			WaitDuration: time.Second,
			EmitStrategy: EmitStrategy("invalid"),
		},
		consumertest.NewNop(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown emit_strategy")
}
