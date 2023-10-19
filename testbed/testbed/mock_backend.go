// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errNonPermanent = errors.New("non permanent error")
var errPermanent = errors.New("permanent error")

type decisionFunc func() error

// MockBackend is a backend that allows receiving the data locally.
type MockBackend struct {
	// Metric and trace consumers
	tc *MockTraceConsumer
	mc *MockMetricConsumer
	lc *MockLogConsumer

	receiver DataReceiver

	// Log file
	logFilePath string
	logFile     *os.File

	// Start/stop flags
	isStarted bool
	stopOnce  sync.Once
	startedAt time.Time

	// Recording fields.
	isRecording     bool
	recordMutex     sync.Mutex
	ReceivedTraces  []ptrace.Traces
	ReceivedMetrics []pmetric.Metrics
	ReceivedLogs    []plog.Logs

	DroppedTraces  []ptrace.Traces
	DroppedMetrics []pmetric.Metrics
	DroppedLogs    []plog.Logs

	LogsToRetry []plog.Logs

	// decision to return permanent/non-permanent errors
	decision decisionFunc
}

// NewMockBackend creates a new mock backend that receives data using specified receiver.
func NewMockBackend(logFilePath string, receiver DataReceiver) *MockBackend {
	mb := &MockBackend{
		logFilePath: logFilePath,
		receiver:    receiver,
		tc:          &MockTraceConsumer{},
		mc:          &MockMetricConsumer{},
		lc:          &MockLogConsumer{},
		decision:    func() error { return nil },
	}
	mb.tc.backend = mb
	mb.mc.backend = mb
	mb.lc.backend = mb
	return mb
}

func (mb *MockBackend) WithDecisionFunc(decision decisionFunc) {
	mb.decision = decision
}

// Start a backend.
func (mb *MockBackend) Start() error {
	log.Printf("Starting mock backend...")

	var err error

	// Open log file
	mb.logFile, err = os.Create(mb.logFilePath)
	if err != nil {
		return err
	}

	err = mb.receiver.Start(mb.tc, mb.mc, mb.lc)
	if err != nil {
		return err
	}

	mb.isStarted = true
	mb.startedAt = time.Now()
	return nil
}

// Stop the backend
func (mb *MockBackend) Stop() {
	mb.stopOnce.Do(func() {
		if !mb.isStarted {
			return
		}

		log.Printf("Stopping mock backend...")

		mb.logFile.Close()
		if err := mb.receiver.Stop(); err != nil {
			log.Printf("Failed to stop receiver: %v", err)
		}
		// Print stats.
		log.Printf("Stopped backend. %s", mb.GetStats())
	})
}

// EnableRecording enables recording of all data received by MockBackend.
func (mb *MockBackend) EnableRecording() {
	mb.recordMutex.Lock()
	defer mb.recordMutex.Unlock()
	mb.isRecording = true
}

func (mb *MockBackend) GetStats() string {
	received := mb.DataItemsReceived()
	return printer.Sprintf("Received:%10d items (%d/sec)", received, int(float64(received)/time.Since(mb.startedAt).Seconds()))
}

// DataItemsReceived returns total number of received spans and metrics.
func (mb *MockBackend) DataItemsReceived() uint64 {
	return mb.tc.numSpansReceived.Load() + mb.mc.numMetricsReceived.Load() + mb.lc.numLogRecordsReceived.Load()
}

// ClearReceivedItems clears the list of received traces and metrics. Note: counters
// return by DataItemsReceived() are not cleared, they are cumulative.
func (mb *MockBackend) ClearReceivedItems() {
	mb.recordMutex.Lock()
	defer mb.recordMutex.Unlock()
	mb.ReceivedTraces = nil
	mb.ReceivedMetrics = nil
	mb.ReceivedLogs = nil
}

func (mb *MockBackend) ConsumeTrace(td ptrace.Traces) {
	mb.recordMutex.Lock()
	defer mb.recordMutex.Unlock()
	if mb.isRecording {
		mb.ReceivedTraces = append(mb.ReceivedTraces, td)
	}
}

func (mb *MockBackend) ConsumeMetric(md pmetric.Metrics) {
	mb.recordMutex.Lock()
	defer mb.recordMutex.Unlock()
	if mb.isRecording {
		mb.ReceivedMetrics = append(mb.ReceivedMetrics, md)
	}
}

var _ consumer.Traces = (*MockTraceConsumer)(nil)

func (mb *MockBackend) ConsumeLogs(ld plog.Logs) {
	if mb.isRecording {
		mb.ReceivedLogs = append(mb.ReceivedLogs, ld)
	}
}

type MockTraceConsumer struct {
	numSpansReceived atomic.Uint64
	backend          *MockBackend
}

func (tc *MockTraceConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (tc *MockTraceConsumer) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	if err := tc.backend.decision(); err != nil {
		if consumererror.IsPermanent(err) && tc.backend.isRecording {
			tc.backend.DroppedTraces = append(tc.backend.DroppedTraces, td)
		}
		return err
	}

	tc.numSpansReceived.Add(uint64(td.SpanCount()))

	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		ils := rs.At(i).ScopeSpans()
		for j := 0; j < ils.Len(); j++ {
			spans := ils.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				var spanSeqnum int64
				var traceSeqnum int64

				seqnumAttr, ok := span.Attributes().Get("load_generator.span_seq_num")
				if ok {
					spanSeqnum = seqnumAttr.Int()
				}

				seqnumAttr, ok = span.Attributes().Get("load_generator.trace_seq_num")
				if ok {
					traceSeqnum = seqnumAttr.Int()
				}

				// Ignore the seqnums for now. We will use them later.
				_ = spanSeqnum
				_ = traceSeqnum

			}
		}
	}

	tc.backend.ConsumeTrace(td)

	return nil
}

var _ consumer.Metrics = (*MockMetricConsumer)(nil)

type MockMetricConsumer struct {
	numMetricsReceived atomic.Uint64
	backend            *MockBackend
}

func (mc *MockMetricConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (mc *MockMetricConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	if err := mc.backend.decision(); err != nil {
		if consumererror.IsPermanent(err) && mc.backend.isRecording {
			mc.backend.DroppedMetrics = append(mc.backend.DroppedMetrics, md)
		}
		return err
	}

	mc.numMetricsReceived.Add(uint64(md.DataPointCount()))
	mc.backend.ConsumeMetric(md)
	return nil
}

func (tc *MockTraceConsumer) MockConsumeTraceData(spanCount int) error {
	tc.numSpansReceived.Add(uint64(spanCount))
	return nil
}

func (mc *MockMetricConsumer) MockConsumeMetricData(metricsCount int) error {
	mc.numMetricsReceived.Add(uint64(metricsCount))
	return nil
}

type MockLogConsumer struct {
	numLogRecordsReceived atomic.Uint64
	backend               *MockBackend
}

func (lc *MockLogConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (lc *MockLogConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	lc.backend.recordMutex.Lock()
	defer lc.backend.recordMutex.Unlock()
	if err := lc.backend.decision(); err != nil {
		if lc.backend.isRecording {
			if consumererror.IsPermanent(err) {
				lc.backend.DroppedLogs = append(lc.backend.DroppedLogs, ld)
			} else {
				lc.backend.LogsToRetry = append(lc.backend.LogsToRetry, ld)
			}
		}
		return err
	}

	recordCount := ld.LogRecordCount()
	lc.numLogRecordsReceived.Add(uint64(recordCount))
	lc.backend.ConsumeLogs(ld)
	return nil
}

// randomNonPermanentError is a decision function that succeeds approximately
// half of the time and fails with a non-permanent error the rest of the time.
func RandomNonPermanentError() error {
	code := codes.Unavailable
	s := status.New(code, errNonPermanent.Error())
	if rand.Float32() < 0.5 {
		return s.Err()
	}
	return nil
}

func GenerateNonPernamentErrorUntil(ch chan bool) error {
	code := codes.Unavailable
	s := status.New(code, errNonPermanent.Error())
	defaultReturn := s.Err()
	if <-ch {
		return defaultReturn
	}
	return nil
}

// randomPermanentError is a decision function that succeeds approximately
// half of the time and fails with a permanent error the rest of the time.
func RandomPermanentError() error {
	if rand.Float32() < 0.5 {
		return consumererror.NewPermanent(errPermanent)
	}
	return nil
}
