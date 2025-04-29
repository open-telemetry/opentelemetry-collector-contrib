// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func TestEndToEnd(t *testing.T) {
	numEntries := 123_456
	numHosts := 4

	ctx := context.Background()
	f := NewFactory(BenchReceiverType{}, component.StabilityLevelUndefined)
	cfg := f.CreateDefaultConfig().(*BenchConfig)
	cfg.NumEntries = numEntries
	cfg.NumHosts = numHosts
	sink := new(consumertest.LogsSink)

	rcvr, err := f.CreateLogs(ctx, receivertest.NewNopSettings(f.Type()), cfg, sink)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, rcvr.Shutdown(context.Background()))
	}()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == numEntries
	}, time.Minute, 100*time.Millisecond)
}

type benchCase struct {
	maxBatchSize  uint
	flushInterval time.Duration
}

func (bc benchCase) run(b *testing.B) {
	for i := 0; i < b.N; i++ {
		f := NewFactory(BenchReceiverType{}, component.StabilityLevelUndefined)
		cfg := f.CreateDefaultConfig().(*BenchConfig)
		cfg.maxBatchSize = bc.maxBatchSize
		cfg.flushInterval = bc.flushInterval
		cfg.NumEntries = numEntries
		cfg.NumHosts = numHosts
		sink := new(consumertest.LogsSink)

		rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, sink)
		require.NoError(b, err)

		b.ReportAllocs()

		require.NoError(b, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		defer func() {
			require.NoError(b, rcvr.Shutdown(context.Background()))
		}()
		require.Eventually(b, func() bool {
			return sink.LogRecordCount() == numEntries
		}, time.Minute, time.Millisecond)
	}
}

// These values establish a consistent baseline for the benchmark.
// They can be tweaked in theory, but it's not clear that varying them
// would add any clarity to comparisons between different benchmarks.
const (
	numEntries = 100_000
	numHosts   = 4
)

func BenchmarkEndToEnd(b *testing.B) {
	// These values may have meaningful performance implications, so benchmarks
	// should cover a variety of values in order to highlight impacts.
	var (
		// emitter
		maxBatchSizes  = []uint{1, 10, 100, 1000, 10_000}
		flushIntervals = []time.Duration{10 * time.Millisecond, 100 * time.Millisecond}
	)

	for _, bs := range maxBatchSizes {
		for _, fi := range flushIntervals {
			name := fmt.Sprintf("maxBatchSize=%d,flushInterval=%s", bs, fi)
			bc := benchCase{maxBatchSize: bs, flushInterval: fi}
			b.Run(name, bc.run)
		}
	}
}

const benchTypeStr = "bench"

var benchType = component.MustNewType(benchTypeStr)

type BenchConfig struct {
	BaseConfig
	BenchOpConfig
}
type BenchReceiverType struct{}

func (f BenchReceiverType) Type() component.Type {
	return benchType
}

func (f BenchReceiverType) CreateDefaultConfig() component.Config {
	return &BenchConfig{
		BaseConfig: BaseConfig{
			Operators: []operator.Config{},
		},
		BenchOpConfig: *NewBenchOpConfig(),
	}
}

func (f BenchReceiverType) BaseConfig(cfg component.Config) BaseConfig {
	return cfg.(*BenchConfig).BaseConfig
}

func (f BenchReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(cfg.(*BenchConfig))
}

func init() {
	operator.Register(benchTypeStr, func() operator.Builder { return NewBenchOpConfig() })
}

// NewBenchOpConfig creates a new benchmarking operator config with default values
func NewBenchOpConfig() *BenchOpConfig {
	return NewBenchOpConfigWithID(benchTypeStr)
}

// NewBenchOpConfigWithID creates a new noop operator config with default values
func NewBenchOpConfigWithID(operatorID string) *BenchOpConfig {
	return &BenchOpConfig{
		InputConfig: helper.NewInputConfig(operatorID, benchTypeStr),
	}
}

// BenchOpConfig is the configuration of a noop operator.
type BenchOpConfig struct {
	helper.InputConfig `mapstructure:",squash"`
	NumEntries         int `mapstructure:"num_entries"`
	NumHosts           int `mapstructure:"num_hosts"`
}

// Build will build a noop operator.
func (c BenchOpConfig) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Input{
		InputOperator: inputOperator,
		entries:       complexEntriesForNDifferentHosts(c.NumEntries, c.NumHosts),
	}, nil
}

// Input is an operator that performs no operations on an entry.
type Input struct {
	helper.InputOperator
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	entries []*entry.Entry
}

// Start will start generating log entries.
func (b *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for n := 0; n < len(b.entries); n++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			err := b.Write(ctx, b.entries[n])
			if err != nil {
				b.Logger().Error("failed to write entry", zap.Error(err))
			}
		}
	}()
	return nil
}

// Stop will stop generating logs.
func (b *Input) Stop() error {
	b.cancel()
	b.wg.Wait()
	return nil
}
