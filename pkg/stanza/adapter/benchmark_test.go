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
	cfg.BenchOpConfig.NumEntries = numEntries
	cfg.BenchOpConfig.NumHosts = numHosts
	sink := new(consumertest.LogsSink)

	rcvr, err := f.CreateLogsReceiver(ctx, receivertest.NewNopCreateSettings(), cfg, sink)
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
	workerCount   int
	maxBatchSize  uint
	flushInterval time.Duration
}

func (bc benchCase) run(b *testing.B) {
	for i := 0; i < b.N; i++ {
		f := NewFactory(BenchReceiverType{}, component.StabilityLevelUndefined)
		cfg := f.CreateDefaultConfig().(*BenchConfig)
		cfg.BaseConfig.numWorkers = bc.workerCount
		cfg.BaseConfig.maxBatchSize = bc.maxBatchSize
		cfg.BaseConfig.flushInterval = bc.flushInterval
		cfg.BenchOpConfig.NumEntries = numEntries
		cfg.BenchOpConfig.NumHosts = numHosts
		sink := new(consumertest.LogsSink)

		rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
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
		// converter
		workerCounts = []int{1, 2, 4, 8, 16}

		// emitter
		maxBatchSizes  = []uint{1, 10, 100, 1000, 10_000}
		flushIntervals = []time.Duration{10 * time.Millisecond, 100 * time.Millisecond}
	)

	for _, wc := range workerCounts {
		for _, bs := range maxBatchSizes {
			for _, fi := range flushIntervals {
				name := fmt.Sprintf("workerCount=%d,maxBatchSize=%d,flushInterval=%s", wc, bs, fi)
				bc := benchCase{workerCount: wc, maxBatchSize: bs, flushInterval: fi}
				b.Run(name, bc.run)
			}
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
	bCfg, _ := newDefaultConfig("").(*BenchOpConfig)
	return &BenchConfig{
		BaseConfig: BaseConfig{
			Operators: []operator.Identifiable{},
		},
		BenchOpConfig: *bCfg,
	}
}

func (f BenchReceiverType) BaseConfig(cfg component.Config) BaseConfig {
	return cfg.(*BenchConfig).BaseConfig
}

func (f BenchReceiverType) InputConfig(cfg component.Config) operator.Identifiable {
	return cfg.(*BenchConfig)
}

func init() {
	operator.RegisterFactory(newBenchOpFactory())
}

func newBenchOpFactory() operator.Factory {
	return operator.NewFactory(benchType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
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

func createOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	c := cfg.(*BenchConfig).BenchOpConfig
	inputOperator, err := c.InputConfig.Build(set.Logger.Sugar())
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
			b.Write(ctx, b.entries[n])
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
