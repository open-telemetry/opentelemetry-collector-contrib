// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package local

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
)

const ms = float64(time.Millisecond)

// Processor implementation for converting incoming UDS receiver messages into
// stats and spanData objects. This processor uses the OpenCensus Go stats
// implementation to handle the PHP stats and converts the PHP span data into
// Go's spanData objects. This allows PHP to have the same featureset as Go
// applications without doing much processing in the critical path and
// reuse Go's OpenCensus Agent Exporter for Stats and Traces.
type Processor struct {
	mu             sync.RWMutex
	droppedTimeout time.Time
	messages       chan *udsreceiver.Message
	measures       map[string]stats.Measure

	logger   log.Logger
	traceExp []trace.Exporter

	closeNotifier chan struct{}
	bufSize       int
	procRoutines  int
	dropCount     int32
}

// New returns a new Processor.
func New(msgBufSize, msgProcRoutines int, exporters []trace.Exporter, logger log.Logger) *Processor {
	registerViews.Do(func() {
		if err := view.Register(
			viewLatency, viewReqCount, viewProcCount, viewDropCount, viewMsgSize,
		); err != nil {
			_ = level.Error(logger).Log("msg", "unable to register internal views", "err", err)
		}
	})
	return &Processor{
		logger:        logger,
		messages:      make(chan *udsreceiver.Message, msgBufSize),
		measures:      make(map[string]stats.Measure),
		traceExp:      exporters,
		closeNotifier: make(chan struct{}),
		procRoutines:  msgProcRoutines,
		bufSize:       msgBufSize,
	}
}

// Run starts our message processor.
func (p *Processor) Run() error {
	var wg sync.WaitGroup

	wg.Add(1 + p.procRoutines)
	go p.notifier(&wg)
	for i := 0; i < p.procRoutines; i++ {
		go p.handleMessages(&wg)
	}
	wg.Wait()

	return nil
}

func (p *Processor) notifier(wg *sync.WaitGroup) {
	defer wg.Done()

	var (
		dcTimeout time.Time
		ticker    = time.NewTicker(1 * time.Second)
	)
	defer ticker.Stop()

	for {
		select {
		case <-p.closeNotifier:
			return
		case <-ticker.C:
			if dc := atomic.LoadInt32(&p.dropCount); dc > 0 && time.Since(dcTimeout) >= 5*time.Second {
				dcTimeout = time.Now()
				_ = level.Error(p.logger).Log(
					"msg", "buffer overflow occurred, dropped messages",
					"dropped", dc,
					"bufsize", p.bufSize,
				)
				atomic.AddInt32(&p.dropCount, -dc)
			}
		}
	}
}

// handleMessages watches for incoming messages on our internal channel and
// dispatches them to the correct handler.
func (p *Processor) handleMessages(wg *sync.WaitGroup) {
	defer wg.Done()

	for m := range p.messages {
		procTime := time.Now()
		msgType := tag.Insert(tagKeyMsgType, m.Type.String())
		_ = stats.RecordWithTags(context.Background(),
			[]tag.Mutator{
				msgType,
				tag.Insert(tagKeyPhase, "queue"),
			},
			msgLatency.M(float64(procTime.Sub(m.ReceiveTime))/ms),
		)
		_ = level.Debug(p.logger).Log("msg", "processing message", "type", m.Type)
		switch m.Type {
		case udsreceiver.MeasureCreate:
			p.createMeasure(m)
		case udsreceiver.ViewReportingPeriod:
			p.reportingPeriod(m)
		case udsreceiver.ViewRegister:
			p.registerView(m)
		case udsreceiver.ViewUnregister:
			p.unregisterView(m)
		case udsreceiver.StatsRecord:
			p.recordStats(m)
		case udsreceiver.TraceExport:
			p.exportSpans(m)
		}
		_ = stats.RecordWithTags(context.Background(),
			[]tag.Mutator{
				msgType,
				tag.Insert(tagKeyPhase, "process"),
			},
			msgProcCount.M(1),
			msgLatency.M(float64(time.Since(procTime))/ms),
		)
	}
}

// Process sends our Message on the internal channel to be processed.
// Returns true on successful ingestion, false on high water mark.
func (p *Processor) Process(m *udsreceiver.Message) bool {
	_ = stats.RecordWithTags(context.Background(),
		[]tag.Mutator{
			tag.Insert(tagKeyMsgType, m.Type.String()),
			tag.Insert(tagKeyPhase, "transport"),
		},
		msgReqCount.M(1),
		msgSize.M(int64(m.MsgLen)),
		msgLatency.M(float64(m.ReceiveTime.Sub(m.StartTime))/ms),
	)
	select {
	case p.messages <- m:
		return true
	default:
		_ = stats.RecordWithTags(context.Background(),
			[]tag.Mutator{tag.Insert(tagKeyMsgType, m.Type.String())},
			msgDropCount.M(1),
		)
		atomic.AddInt32(&p.dropCount, 1)
		return false
	}
}

// Close shuts down our Processor.
func (p *Processor) Close() error {
	close(p.closeNotifier)
	close(p.messages)
	return nil
}

func (p *Processor) createMeasure(m *udsreceiver.Message) bool {
	defer func() {
		if recover() != nil {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
	}()

	mType := udsreceiver.MeasurementType(m.RawPayload[0])
	idx := 1

	name, n := decodeString(m.RawPayload[idx:])
	if n < 0 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}
	idx += n

	description, n := decodeString(m.RawPayload[idx:])
	if n < 0 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}
	idx += n

	unit, n := decodeString(m.RawPayload[idx:])
	if n < 0 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}

	switch mType {
	case udsreceiver.TypeInt:
		p.mu.Lock()
		if _, ok := p.measures[name]; !ok {
			p.measures[name] = stats.Int64(name, description, unit)
		}
		p.mu.Unlock()
		return true
	case udsreceiver.TypeFloat:
		p.mu.Lock()
		if _, ok := p.measures[name]; !ok {
			p.measures[name] = stats.Float64(name, description, unit)
		}
		p.mu.Unlock()
		return true
	default:
		_ = level.Debug(p.logger).Log("msg", "unknown measure type", "type", int(mType))
		return false
	}
}

func (p *Processor) reportingPeriod(m *udsreceiver.Message) bool {
	defer func() {
		if recover() != nil {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
	}()

	f, _ := decodeFloat(m, m.RawPayload)
	msec := time.Duration(f * 1e3)
	view.SetReportingPeriod(msec * time.Millisecond)

	return true
}

func (p *Processor) registerView(m *udsreceiver.Message) bool {
	var (
		n     int
		ok    bool
		err   error
		views []*view.View
	)

	defer func() {
		if recover() != nil {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
	}()

	viewCount, idx := binary.Uvarint(m.RawPayload)
	if idx < 1 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}

	for i := uint64(0); i < viewCount; i++ {
		var v view.View

		v.Name, n = decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		v.Description, n = decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		tagKeyCount, n := binary.Uvarint(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		for j := uint64(0); j < tagKeyCount; j++ {
			name, n := decodeString(m.RawPayload[idx:])
			if n < 1 {
				_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
				return false
			}
			idx += n

			tagKey, err := tag.NewKey(name)
			if err != nil {
				_ = level.Error(p.logger).Log("msg", "register views failed on creating tag key", "err", err)
				return false
			}
			v.TagKeys = append(v.TagKeys, tagKey)
		}

		measureName, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		p.mu.RLock()
		v.Measure, ok = p.measures[measureName]
		p.mu.RUnlock()

		if !ok {
			_ = level.Error(p.logger).Log("msg", "register views failed on unknown measure", "err", err)
			return false
		}

		aggregationType, n := binary.Uvarint(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		switch view.AggType(aggregationType) {
		case view.AggTypeCount:
			v.Aggregation = view.Count()
		case view.AggTypeSum:
			v.Aggregation = view.Sum()
		case view.AggTypeDistribution:
			boundaryCount, n := binary.Uvarint(m.RawPayload[idx:])
			if n < 1 {
				_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
				return false
			}
			idx += n
			var boundaries []float64
			for k := uint64(0); k < boundaryCount; k++ {
				boundary, n := decodeFloat(m, m.RawPayload[idx:])
				boundaries = append(boundaries, boundary)
				idx += n
			}
			v.Aggregation = view.Distribution(boundaries...)
		case view.AggTypeLastValue:
			v.Aggregation = view.LastValue()
		default:
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}

		views = append(views, &v)
	}

	if err := view.Register(views...); err != nil {
		_ = level.Error(p.logger).Log("msg", "register views failed.", "err", err)
		return false
	}

	return true
}

func (p *Processor) unregisterView(m *udsreceiver.Message) bool {
	defer func() {
		if recover() != nil {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
	}()

	viewCount, idx := binary.Uvarint(m.RawPayload)
	if idx < 1 || viewCount == 0 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
	}

	var views []*view.View
	for i := uint64(0); i < viewCount; i++ {
		name, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
		views = append(views, &view.View{Name: name})
	}
	view.Unregister(views...)

	return true
}

func (p *Processor) recordStats(m *udsreceiver.Message) bool {
	var (
		n            int
		mType        uint64
		measurements []*measurement
		mutators     []tag.Mutator
		attachments  = make(metricdata.Attachments)
		ms           []stats.Measurement
	)

	defer func() {
		if recover() != nil {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		}
	}()

	measurementCount, idx := binary.Uvarint(m.RawPayload)
	if idx < 1 || measurementCount == 0 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}

	for i := uint64(0); i < measurementCount; i++ {
		ms := &measurement{}
		ms.name, n = decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		mType, n = binary.Uvarint(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		ms.mType = udsreceiver.MeasurementType(mType)
		idx += n

		switch ms.mType {
		case udsreceiver.TypeInt:
			ms.val, n = binary.Uvarint(m.RawPayload[idx:])
		case udsreceiver.TypeFloat:
			ms.val, n = decodeFloat(m, m.RawPayload[idx:])
		default:
			_ = level.Warn(p.logger).Log("msg", "unknown measurement type encountered", "type", ms.mType)
		}
		idx += n
		measurements = append(measurements, ms)
	}

	tagCount, n := binary.Uvarint(m.RawPayload[idx:])
	if n < 1 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}
	idx += n

	for i := uint64(0); i < tagCount; i++ {
		key, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		value, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		tagKey, err := tag.NewKey(key)
		if err != nil {
			_ = level.Error(p.logger).Log("msg", "invalid tag payload encountered", "err", err)
			return false
		}
		mutators = append(mutators, tag.Insert(tagKey, value))
	}

	attachmentCount, n := binary.Uvarint(m.RawPayload[idx:])
	if n < 1 {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
		return false
	}
	idx += n

	for i := uint64(0); i < attachmentCount; i++ {
		key, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n

		value, n := decodeString(m.RawPayload[idx:])
		if n < 1 {
			_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered")
			return false
		}
		idx += n
		attachments[key] = value
	}

	ctx := context.Background()

	if len(attachments) > 0 {
		ctx = udsreceiver.AttachmentsToContext(ctx, attachments)
	}

	ms = p.processMeasurement(measurements)
	if len(ms) == 0 {
		_ = level.Error(p.logger).Log("msg", "invalid message payload encountered", "err", "no measurements recorded")
		return false
	}

	if err := stats.RecordWithTags(ctx, mutators, ms...); err != nil {
		_ = level.Error(p.logger).Log("msg", "invalid tags in record context", "err", err)
		return false
	}

	return true
}

func (p *Processor) exportSpans(m *udsreceiver.Message) bool {
	if len(p.traceExp) == 0 {
		// no need to parse if not exported
		return true
	}

	var spans []span
	if err := json.Unmarshal(m.RawPayload, &spans); err != nil {
		_ = level.Warn(p.logger).Log("msg", "invalid message payload encountered", "err", err)
		return false
	}
	for _, span := range spans {
		traceID, ok := b3.ParseTraceID(span.TraceID)
		if !ok {
			continue
		}
		spanID, ok := b3.ParseSpanID(span.SpanID)
		if !ok {
			continue
		}
		parentSpanID, _ := b3.ParseSpanID(span.ParentSpanID)

		s := &trace.SpanData{
			SpanContext: trace.SpanContext{
				TraceID: traceID,
				SpanID:  spanID,
				// TODO: TraceOptions
				// TODO: Tracestate
			},
			ParentSpanID: parentSpanID,
			SpanKind:     trace.SpanKindUnspecified,
			Name:         span.Name,
			// TODO: optionally annotate with span.StackTrace
			StartTime:  time.Time(span.StartTime),
			EndTime:    time.Time(span.EndTime),
			Attributes: make(map[string]interface{}),
			// TODO: Annotations (timeEvents)
			// TODO: MessageEvent
			Status: trace.Status(span.Status),
			// TODO: Links
			HasRemoteParent: !span.SameProcess,
		}
		switch span.Kind {
		case "CLIENT":
			s.SpanKind = trace.SpanKindClient
		case "SERVER":
			s.SpanKind = trace.SpanKindServer
		}
		for key, val := range span.Attributes {
			s.Attributes[key] = val
		}

		for _, exporter := range p.traceExp {
			exporter.ExportSpan(s)
		}

	}
	return true
}

func (p *Processor) processMeasurement(ms []*measurement) []stats.Measurement {
	var measurements []stats.Measurement

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, m := range ms {
		mm, ok := p.measures[m.name]
		if !ok {
			_ = level.Debug(p.logger).Log("msg", "measure not found", "name", m.name)
			continue
		}

		switch m.mType {
		case udsreceiver.TypeInt:
			if im, ok := mm.(*stats.Int64Measure); ok {
				if val, ok := m.val.(uint64); ok {
					measurements = append(measurements, im.M(int64(val)))
					continue
				}
			}
			_ = level.Debug(p.logger).Log("msg", "expected measurement or value type not found", "name", m.name)
		case udsreceiver.TypeFloat:
			if fm, ok := mm.(*stats.Float64Measure); ok {
				if val, ok := m.val.(float64); ok {
					measurements = append(measurements, fm.M(val))
					continue
				}
			}
			_ = level.Debug(p.logger).Log("msg", "expected float measurement or value type not found", "name", m.name)
		default:
			_ = level.Debug(p.logger).Log("msg", "unknown measurement type", "type", m.mType)
		}
	}

	return measurements
}

func decodeString(buf []byte) (string, int) {
	if len(buf) < 1 {
		return "", -1
	}
	size, n := binary.Uvarint(buf)
	i := int(size) + n
	if n < 1 || len(buf) < i {
		return "", -1
	}

	return string(buf[n:i]), i
}

func decodeFloat(m *udsreceiver.Message, buf []byte) (float64, int) {
	if m.Float32 {
		return float64(math.Float32frombits(binary.BigEndian.Uint32(buf))), 4
	}
	return math.Float64frombits(binary.BigEndian.Uint64(buf)), 8
}
