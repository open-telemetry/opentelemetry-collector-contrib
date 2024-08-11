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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

func TestEventCallback(t *testing.T) {
	set := processortest.NewNopSettings()
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
	set := processortest.NewNopSettings()
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
	set := processortest.NewNopSettings()
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
	set := processortest.NewNopSettings()
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
	set := processortest.NewNopSettings()
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
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < 30; j++ {
						assert.Equal(t, realTraceID, workerIndexForTraceID(pcommon.TraceID(tt.traceID), 100))
					}
				}()
			}
			wg.Wait()
		})
	}
}

func TestEventShutdown(t *testing.T) {
	set := processortest.NewNopSettings()
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

	time.Sleep(10 * time.Millisecond)  // give it a bit of time to process the items
	assert.Equal(t, 1, em.numEvents()) // we should have one pending event in the queue, the second traceRemoved event

	shutdownWg := sync.WaitGroup{}
	shutdownWg.Add(1)
	go func() {
		em.shutdown()
		shutdownWg.Done()
	}()

	wg.Done()                          // the pending event should be processed
	time.Sleep(100 * time.Millisecond) // give it a bit of time to process the items

	assert.Equal(t, 0, em.numEvents())

	// new events should *not* be processed
	em.workers[0].fire(event{
		typ:     traceExpired,
		payload: pcommon.TraceID([16]byte{1, 2, 3, 4}),
	})

	// verify
	assert.Equal(t, int64(1), traceReceivedFired.Load())

	// If the code is wrong, there's a chance that the test will still pass
	// in case the event is processed after the assertion.
	// for this reason, we add a small delay here
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, int64(0), traceExpiredFired.Load())

	// wait until the shutdown has returned
	shutdownWg.Wait()
}

func TestPeriodicMetrics(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(s.NewSettings().TelemetrySettings)
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
	assert.Eventually(t, func() bool {
		return getGaugeValue(t, "otelcol_processor_groupbytrace_num_events_in_queue", s) == 1
	}, 1*time.Second, 10*time.Millisecond)

	wg.Done() // release all events

	// ensure our gauge is now showing no items in the queue
	assert.Eventually(t, func() bool {
		return getGaugeValue(t, "otelcol_processor_groupbytrace_num_events_in_queue", s) == 0
	}, 1*time.Second, 10*time.Millisecond)

	// signal and wait for the recursive call to finish
	em.shutdownLock.Lock()
	em.closed = true
	em.shutdownLock.Unlock()
	time.Sleep(5 * time.Millisecond)
}

func TestForceShutdown(t *testing.T) {
	set := processortest.NewNopSettings()
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
	assert.True(t, duration > 20*time.Millisecond)

	// wait for shutdown goroutine to end
	time.Sleep(100 * time.Millisecond)
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

	// test
	succeed, err := doWithTimeout(20*time.Millisecond, func() error {
		time.Sleep(1 * time.Second)
		return nil
	})
	assert.False(t, succeed)
	assert.NoError(t, err)

	// verify
	assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond)
}

func getGaugeValue(t *testing.T, name string, tt componentTestTelemetry) int64 {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))
	m := tt.getMetric(name, md).Data
	g := m.(metricdata.Gauge[int64])
	assert.Len(t, g.DataPoints, 1, "expected exactly one data point")
	return g.DataPoints[0].Value
}

func assertGaugeNotCreated(t *testing.T, name string, tt componentTestTelemetry) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))
	got := tt.getMetric(name, md)
	assert.Equal(t, got, metricdata.Metrics{}, "gauge exists already but shouldn't")
}
