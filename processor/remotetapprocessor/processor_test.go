// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor/internal/metadata"
)

func TestConsumeMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32967")
	}
	metric := pmetric.NewMetrics()
	metric.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")

	cases := []struct {
		name  string
		limit int
	}{
		{name: "limit_0", limit: 0},
		{name: "limit_1", limit: 1},
		{name: "limit_10", limit: 10},
		{name: "limit_30", limit: 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf := &Config{
				Limit: rate.Limit(c.limit),
			}

			processor := newProcessor(processortest.NewNopSettings(metadata.Type), conf)

			ch := make(chan []byte)
			idx := processor.cs.add(ch)
			receiveNum := 0
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range ch {
					receiveNum++
				}
			}()

			for i := 0; i < c.limit*2; i++ {
				// send metric to chan c.limit*2 per sec.
				metric2, err := processor.ConsumeMetrics(context.Background(), metric)
				assert.NoError(t, err)
				assert.Equal(t, metric, metric2)
			}

			processor.cs.closeAndRemove(idx)
			wg.Wait()
			assert.Equal(t, c.limit, receiveNum)
		})
	}
}

func TestConsumeLogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32967")
	}
	log := plog.NewLogs()
	log.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("foo")

	cases := []struct {
		name  string
		limit int
	}{
		{name: "limit_0", limit: 0},
		{name: "limit_1", limit: 1},
		{name: "limit_10", limit: 10},
		{name: "limit_30", limit: 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf := &Config{
				Limit: rate.Limit(c.limit),
			}

			processor := newProcessor(processortest.NewNopSettings(metadata.Type), conf)

			ch := make(chan []byte)
			idx := processor.cs.add(ch)
			receiveNum := 0
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range ch {
					receiveNum++
				}
			}()

			// send log to chan c.limit*2 per sec.
			for i := 0; i < c.limit*2; i++ {
				log2, err := processor.ConsumeLogs(context.Background(), log)
				assert.NoError(t, err)
				assert.Equal(t, log, log2)
			}

			processor.cs.closeAndRemove(idx)
			wg.Wait()
			t.Log(receiveNum)
			assert.Equal(t, c.limit, receiveNum)
		})
	}
}

func TestConsumeTraces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32967")
	}

	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")

	cases := []struct {
		name  string
		limit int
	}{
		{name: "limit_0", limit: 0},
		{name: "limit_1", limit: 1},
		{name: "limit_10", limit: 10},
		{name: "limit_30", limit: 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf := &Config{
				Limit: rate.Limit(c.limit),
			}

			processor := newProcessor(processortest.NewNopSettings(metadata.Type), conf)

			ch := make(chan []byte)
			idx := processor.cs.add(ch)
			receiveNum := 0
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range ch {
					receiveNum++
				}
			}()

			for i := 0; i < c.limit*2; i++ {
				// send trace to chan c.limit*2 per sec.
				trace2, err := processor.ConsumeTraces(context.Background(), trace)
				assert.NoError(t, err)
				assert.Equal(t, trace, trace2)
			}

			processor.cs.closeAndRemove(idx)
			wg.Wait()
			assert.Equal(t, c.limit, receiveNum)
		})
	}
}
