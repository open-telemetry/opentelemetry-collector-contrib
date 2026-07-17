// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

func TestEventCallback(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)

	for _, tt := range []struct {
		casename         string
		typ              eventType
		payload          any
		registerCallback func(em *eventMachine, wg *sync.WaitGroup)
	}{
		{
			casename: "onTraceReceived",
			typ:      traceReceived,
			payload:  tracesWithID{id: pcommon.NewTraceIDEmpty(), td: ptrace.NewTraces()},
			registerCallback: func(em *eventMachine, wg *sync.WaitGroup) {
				em.onTraceReceived = func(_ tracesWithID, _ *eventMachineWorker) error {
					wg.Done()
					return nil
				}
			},
		},
		{
			casename: "onTraceExpired",
			typ:      traceExpired,
			payload:  pcommon.TraceID([16]byte{1, 2, 3, 4}),
			registerCallback: func(em *eventMachine, wg *sync.WaitGroup) {
				em.onTraceExpired = func(expired pcommon.TraceID, _ *eventMachineWorker) error {
					wg.Done()
					assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), expired)
					return nil
				}
			},
		},
		{
			casename: "onTraceReleased",
			typ:      traceReleased,
			payload:  []ptrace.ResourceSpans{},
			registerCallback: func(em *eventMachine, wg *sync.WaitGroup) {
				em.onTraceReleased = func(_ []ptrace.ResourceSpans) error {
					wg.Done()
					return nil
				}
			},
		},
		{
			casename: "onTraceRemoved",
			typ:      traceRemoved,
			payload:  pcommon.TraceID([16]byte{1, 2, 3, 4}),
			registerCallback: func(em *eventMachine, wg *sync.WaitGroup) {
				em.onTraceRemoved = func(expired pcommon.TraceID) error {
					wg.Done()
					assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), expired)
					return nil
				}
			},
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			// prepare
			logger, err := zap.NewDevelopment()
			require.NoError(t, err)

			wg := &sync.WaitGroup{}
			em := newEventMachine(logger, 50, 1, 1_000, tel)
			tt.registerCallback(em, wg)

			em.startInBackground()
			defer em.shutdown()

			// test
			wg.Add(1)
			em.workers[0].fire(event{
				typ:     tt.typ,
				payload: tt.payload,
			})

			// verify
			wg.Wait()
		})
	}
}

func TestEventCallbackNotSet(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	for _, tt := range []struct {
		casename string
		typ      eventType
	}{
		{
			casename: "onTraceReceived",
			typ:      traceReceived,
		},
		{
			casename: "onTraceExpired",
			typ:      traceExpired,
		},
		{
			casename: "onTraceReleased",
			typ:      traceReleased,
		},
		{
			casename: "onTraceRemoved",
			typ:      traceRemoved,
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			// prepare
			logger, err := zap.NewDevelopment()
			require.NoError(t, err)

			wg := &sync.WaitGroup{}
			em := newEventMachine(logger, 50, 1, 1_000, tel)
			em.onError = func(_ event) {
				wg.Done()
			}
			em.startInBackground()
			defer em.shutdown()

			// test
			wg.Add(1)
			em.workers[0].fire(event{
				typ: tt.typ,
			})

			// verify
			wg.Wait()
		})
	}
}

func TestEventInvalidPayload(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	for _, tt := range []struct {
		casename         string
		typ              eventType
		registerCallback func(*eventMachine, *sync.WaitGroup)
	}{
		{
			casename: "onTraceReceived",
			typ:      traceReceived,
			registerCallback: func(em *eventMachine, _ *sync.WaitGroup) {
				em.onTraceReceived = func(_ tracesWithID, _ *eventMachineWorker) error {
					return nil
				}
			},
		},
		{
			casename: "onTraceExpired",
			typ:      traceExpired,
			registerCallback: func(em *eventMachine, _ *sync.WaitGroup) {
				em.onTraceExpired = func(_ pcommon.TraceID, _ *eventMachineWorker) error {
					return nil
				}
			},
		},
		{
			casename: "onTraceReleased",
			typ:      traceReleased,
			registerCallback: func(em *eventMachine, _ *sync.WaitGroup) {
				em.onTraceReleased = func(_ []ptrace.ResourceSpans) error {
					return nil
				}
			},
		},
		{
			casename: "onTraceRemoved",
			typ:      traceRemoved,
			registerCallback: func(em *eventMachine, _ *sync.WaitGroup) {
				em.onTraceRemoved = func(_ pcommon.TraceID) error {
					return nil
				}
			},
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			// prepare
			logger, err := zap.NewDevelopment()
			require.NoError(t, err)

			wg := &sync.WaitGroup{}
			em := newEventMachine(logger, 50, 1, 1_000, tel)
			em.onError = func(_ event) {
				wg.Done()
			}
			tt.registerCallback(em, wg)
			em.startInBackground()
			defer em.shutdown()

			// test
			wg.Add(1)
			em.workers[0].fire(event{
				typ: tt.typ,
			})

			// verify
			wg.Wait()
		})
	}
}

func TestEventUnknownType(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	// prepare
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	em := newEventMachine(logger, 50, 1, 1_000, tel)
	em.onError = func(_ event) {
		wg.Done()
	}
	em.startInBackground()
	defer em.shutdown()

	// test
	wg.Add(1)
	em.workers[0].fire(event{
		typ: eventType(1234),
	})

	// verify
	wg.Wait()
}

func TestEventTracePerWorker(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	for _, tt := range []struct {
		casename  string
		traceID   [16]byte
		errString string
	}{
		{
			casename:  "invalid traceID",
			errString: "eventmachine consume failed:",
		},

		{
			casename: "traceID 1",
			traceID:  [16]byte{1},
		},

		{
			casename: "traceID 2",
			traceID:  [16]byte{2},
		},

		{
			casename: "traceID 3",
			traceID:  [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			em := newEventMachine(zap.NewNop(), 200, 100, 1_000, tel)

			var wg sync.WaitGroup
			var workerForTrace *eventMachineWorker
			em.onTraceReceived = func(_ tracesWithID, w *eventMachineWorker) error {
				workerForTrace = w
				w.fire(event{
					typ:     traceExpired,
					payload: pcommon.TraceID([16]byte{1}),
				})
				return nil
			}
			em.onTraceExpired = func(_ pcommon.TraceID, w *eventMachineWorker) error {
				assert.Equal(t, workerForTrace, w)
				wg.Done()
				return nil
			}
			em.startInBackground()
			defer em.shutdown()

			td := ptrace.NewTraces()
			ils := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
			if tt.traceID != [16]byte{} {
				span := ils.Spans().AppendEmpty()
				span.SetTraceID(tt.traceID)
			}

			// test
			wg.Add(1)
			err := em.consume(td)

			// verify
			if tt.errString == "" {
				require.NoError(t, err)
			} else {
				wg.Done()
				require.Truef(t, strings.HasPrefix(err.Error(), tt.errString), "error should have prefix %q", tt.errString)
			}

			wg.Wait()
		})
	}
}

func TestEventConsumeConsistency(t *testing.T) {
	for _, tt := range []struct {
		casename string
		traceID  [16]byte
	}{
		{
			casename: "trace 1",
			traceID:  [16]byte{1, 2, 3, 4},
		},

		{
			casename: "trace 2",
			traceID:  [16]byte{2, 3, 4, 5},
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			realTraceID := workerIndexForTraceID(pcommon.TraceID(tt.traceID), 100)
			var wg sync.WaitGroup
			for range 50 {
				wg.Go(func() {
					for range 30 {
						assert.Equal(t, realTraceID, workerIndexForTraceID(pcommon.TraceID(tt.traceID), 100))
					}
				})
			}
			wg.Wait()
		})
	}
}

func TestEventShutdown(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	// prepare
	wg := sync.WaitGroup{}
	wg.Add(1)

	traceReceivedFired := &atomic.Int64{}
	traceExpiredFired := &atomic.Int64{}
	em := newEventMachine(zap.NewNop(), 50, 1, 1_000, tel)
	em.onTraceReceived = func(tracesWithID, *eventMachineWorker) error {
		traceReceivedFired.Store(1)
		return nil
	}
	em.onTraceExpired = func(pcommon.TraceID, *eventMachineWorker) error {
		traceExpiredFired.Store(1)
		return nil
	}
	em.onTraceRemoved = func(pcommon.TraceID) error {
		wg.Wait()
		return nil
	}
	em.startInBackground()

	// test
	em.workers[0].fire(event{
		typ:     traceReceived,
		payload: tracesWithID{id: pcommon.NewTraceIDEmpty(), td: ptrace.NewTraces()},
	})
	em.workers[0].fire(event{
		typ:     traceRemoved,
		payload: pcommon.TraceID([16]byte{1, 2, 3, 4}),
	})
	em.workers[0].fire(event{
		typ:     traceRemoved,
		payload: pcommon.TraceID([16]byte{1, 2, 3, 4}),
	})

	// wait for events to process - we should have one pending event in the queue, the second traceRemoved event
	assert.Eventually(t, func() bool {
		return em.numEvents() == 1
	}, 1*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, em.numEvents())

	shutdownWg := sync.WaitGroup{}
	shutdownWg.Go(func() {
		em.shutdown()
	})

	wg.Done() // the pending event should be processed
	// wait for shutdown to process remaining events
	assert.Eventually(t, func() bool {
		return em.numEvents() == 0
	}, 1*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, em.numEvents())

	// verify
	assert.Equal(t, int64(1), traceReceivedFired.Load())

	// wait until the shutdown has returned
	shutdownWg.Wait()

	// new events should *not* be processed after shutdown
	em.workers[0].fire(event{
		typ:     traceExpired,
		payload: pcommon.TraceID([16]byte{1, 2, 3, 4}),
	})

	// If the code is wrong, there's a chance that the test will still pass
	// in case the event is processed after the assertion.
	// Verify that the expired event is not processed (should remain 0)
	assert.Eventually(t, func() bool {
		return traceExpiredFired.Load() == 0
	}, 100*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, int64(0), traceExpiredFired.Load())
}

func TestEventShutdownMultipleWorkers(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)

	const numWorkers = 10
	traceReceivedCount := &atomic.Int64{}
	traceExpiredCount := &atomic.Int64{}

	em := newEventMachine(zap.NewNop(), 50, numWorkers, 1_000, tel)
	em.onTraceReceived = func(tracesWithID, *eventMachineWorker) error {
		traceReceivedCount.Add(1)
		return nil
	}
	em.onTraceExpired = func(pcommon.TraceID, *eventMachineWorker) error {
		traceExpiredCount.Add(1)
		return nil
	}
	em.startInBackground()

	for i := range numWorkers {
		em.workers[i].fire(event{
			typ:     traceReceived,
			payload: tracesWithID{id: pcommon.NewTraceIDEmpty(), td: ptrace.NewTraces()},
		})
	}

	assert.Eventually(t, func() bool {
		return traceReceivedCount.Load() == numWorkers
	}, 1*time.Second, 10*time.Millisecond)

	shutdownWg := sync.WaitGroup{}
	shutdownWg.Go(func() {
		em.shutdown()
	})

	shutdownWg.Wait()

	// Fire events to all workers after shutdown - none should be processed
	for i := range numWorkers {
		em.workers[i].fire(event{
			typ:     traceExpired,
			payload: pcommon.TraceID([16]byte{byte(i)}),
		})
	}

	assert.Eventually(t, func() bool {
		return traceExpiredCount.Load() == 0
	}, 100*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, int64(0), traceExpiredCount.Load(), "no traceExpired events should be processed after shutdown")
}

func TestPeriodicMetrics(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	t.Cleanup(func() {
		require.NoError(t, s.Shutdown(t.Context()))
	})
	telemetryBuilder, err := metadata.NewTelemetryBuilder(s.newTelemetrySettings())
	require.NoError(t, err)

	em := newEventMachine(zap.NewNop(), 50, 1, 1_000, telemetryBuilder)
	em.metricsCollectionInterval = time.Millisecond

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		expected := 2
		calls := 0
		for range em.workers[0].events {
			// we expect two events, after which we just exit the loop
			// if we return from here, we'd still have one item in the queue that is not going to be consumed
			wg.Wait()
			calls++

			if calls == expected {
				return
			}
		}
	}()

	// sanity check
	assertGaugeNotCreated(t, "otelcol_processor_groupbytrace_num_events_in_queue", s)

	// test
	em.workers[0].fire(event{typ: traceReceived})
	em.workers[0].fire(event{typ: traceReceived}) // the first is consumed right away, the second is in the queue
	go em.periodicMetrics()

	// ensure our gauge is showing 1 item in the queue
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		val := getGaugeValue(t.Context(), tt, "otelcol_processor_groupbytrace_num_events_in_queue", s)
		if val == -1 {
			tt.Errorf("gauge not yet created or has no data points")
			return
		}
		assert.Equal(tt, int64(1), val)
	}, 1*time.Second, 10*time.Millisecond)

	wg.Done() // release all events

	// ensure our gauge is now showing no items in the queue
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		val := getGaugeValue(t.Context(), tt, "otelcol_processor_groupbytrace_num_events_in_queue", s)
		if val == -1 {
			tt.Errorf("gauge not yet created or has no data points")
			return
		}
		assert.Equal(tt, int64(0), val)
	}, 1*time.Second, 10*time.Millisecond)

	// signal and wait for the recursive call to finish
	em.shutdownLock.Lock()
	em.closed = true
	em.shutdownLock.Unlock()
	// Wait for periodicMetrics to detect the closed flag and return
	assert.Eventually(t, func() bool {
		em.shutdownLock.RLock()
		defer em.shutdownLock.RUnlock()
		return em.closed
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func TestForceShutdown(t *testing.T) {
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	// prepare
	em := newEventMachine(zap.NewNop(), 50, 1, 1_000, tel)
	em.shutdownTimeout = 20 * time.Millisecond

	// test
	em.workers[0].fire(event{typ: traceExpired})

	start := time.Now()
	em.shutdown() // should take about 20ms to return
	duration := time.Since(start)

	// verify
	assert.Greater(t, duration, 20*time.Millisecond)

	// Verify shutdown completed - the machine should be closed
	assert.Eventually(t, func() bool {
		em.shutdownLock.RLock()
		defer em.shutdownLock.RUnlock()
		return em.closed
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func TestDoWithTimeout_NoTimeout(t *testing.T) {
	// prepare
	wantErr := errors.New("my error")
	// test
	succeed, err := doWithTimeout(20*time.Millisecond, func() error {
		return wantErr
	})
	assert.True(t, succeed)
	assert.Equal(t, wantErr, err)
}

func TestDoWithTimeout_TimeoutTrigger(t *testing.T) {
	// prepare
	start := time.Now()
	blockCh := make(chan struct{})
	defer close(blockCh) // cleanup: unblock the goroutine to prevent leak

	// test
	succeed, err := doWithTimeout(20*time.Millisecond, func() error {
		<-blockCh // block until timeout or channel closed
		return nil
	})
	assert.False(t, succeed)
	assert.NoError(t, err)

	// verify
	assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond)
}

func getGaugeValue(ctx context.Context, t *assert.CollectT, name string, tt testTelemetry) int64 {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(ctx, &md))
	metric := tt.getMetric(name, md)
	if metric == (metricdata.Metrics{}) {
		return -1 // return sentinel value to indicate metric doesn't exist yet
	}
	m := metric.Data
	var g metricdata.Gauge[int64]
	var ok bool
	if g, ok = m.(metricdata.Gauge[int64]); !ok {
		return -1 // return sentinel value to indicate gauge data is missing
	}
	if len(g.DataPoints) == 0 {
		return -1 // return sentinel value to indicate no data points yet
	}
	return g.DataPoints[0].Value
}

func assertGaugeNotCreated(t *testing.T, name string, tt testTelemetry) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(t.Context(), &md))
	got := tt.getMetric(name, md)
	assert.Equal(t, metricdata.Metrics{}, got, "gauge exists already but shouldn't")
}

type testTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func setupTestTelemetry() testTelemetry {
	reader := sdkmetric.NewManualReader()
	return testTelemetry{
		reader:        reader,
		meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	}
}

func (tt *testTelemetry) newTelemetrySettings() component.TelemetrySettings {
	set := componenttest.NewNopTelemetrySettings()
	set.MeterProvider = tt.meterProvider
	return set
}

func (*testTelemetry) getMetric(name string, got metricdata.ResourceMetrics) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	return metricdata.Metrics{}
}

func (tt *testTelemetry) Shutdown(ctx context.Context) error {
	return tt.meterProvider.Shutdown(ctx)
}
