// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

type memoryStorage struct {
	sync.RWMutex
	content                   map[pcommon.TraceID][]ptrace.ResourceSpans
	telemetry                 *metadata.TelemetryBuilder
	stopped                   bool
	stoppedLock               sync.RWMutex
	metricsCollectionInterval time.Duration
}

var _ storage = (*memoryStorage)(nil)

func newMemoryStorage(telemetry *metadata.TelemetryBuilder) *memoryStorage {
	return &memoryStorage{
		content:                   make(map[pcommon.TraceID][]ptrace.ResourceSpans),
		metricsCollectionInterval: time.Second,
		telemetry:                 telemetry,
	}
}

func (st *memoryStorage) createOrAppend(traceID pcommon.TraceID, td ptrace.Traces) error {
	st.Lock()
	defer st.Unlock()

	// getting zero value is fine
	content := st.content[traceID]

	newRss := ptrace.NewResourceSpansSlice()
	td.ResourceSpans().CopyTo(newRss)
	for i := 0; i < newRss.Len(); i++ {
		content = append(content, newRss.At(i))
	}
	st.content[traceID] = content

	return nil
}

func (st *memoryStorage) get(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
	st.RLock()
	rss, ok := st.content[traceID]
	st.RUnlock()
	if !ok {
		return nil, nil
	}

	result := make([]ptrace.ResourceSpans, len(rss))
	for i, rs := range rss {
		newRS := ptrace.NewResourceSpans()
		rs.CopyTo(newRS)
		result[i] = newRS
	}

	return result, nil
}

// delete will return a reference to a ResourceSpans. Changes to the returned object may not be applied
// to the version in the storage.
func (st *memoryStorage) delete(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
	st.Lock()
	defer st.Unlock()

	defer delete(st.content, traceID)
	return st.content[traceID], nil
}

func (st *memoryStorage) start() error {
	go st.periodicMetrics()
	return nil
}

func (st *memoryStorage) shutdown() error {
	st.stoppedLock.Lock()
	defer st.stoppedLock.Unlock()
	st.stopped = true
	return nil
}

func (st *memoryStorage) periodicMetrics() {
	numTraces := st.count()
	st.telemetry.ProcessorGroupbytraceNumTracesInMemory.Record(context.Background(), int64(numTraces))

	st.stoppedLock.RLock()
	stopped := st.stopped
	st.stoppedLock.RUnlock()
	if stopped {
		return
	}

	time.AfterFunc(st.metricsCollectionInterval, func() {
		st.periodicMetrics()
	})
}

func (st *memoryStorage) count() int {
	st.RLock()
	defer st.RUnlock()
	return len(st.content)
}
