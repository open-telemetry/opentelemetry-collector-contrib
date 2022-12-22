// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

var (
	errOverCapacity = errors.New("over capacity")
)

// Minimum number of bytes to compress. 1500 is the MTU of an ethernet frame.
const minCompressionLen = 1500

// client sends the data to the splunk backend.
type client struct {
	config         *Config
	url            *url.URL
	healthCheckURL *url.URL
	client         *http.Client
	logger         *zap.Logger
	wg             sync.WaitGroup
	headers        map[string]string
}

// bufferState encapsulates intermediate buffer state when pushing data
type bufferState struct {
	compressionAvailable bool
	compressionEnabled   bool
	bufferMaxLen         uint
	writer               io.Writer
	buf                  *bytes.Buffer
	bufFront             *index
	resource             int
	library              int
}

func (b *bufferState) reset() {
	b.buf.Reset()
	b.compressionEnabled = false
	b.writer = &cancellableBytesWriter{innerWriter: b.buf, maxCapacity: b.bufferMaxLen}
}

func (b *bufferState) Read(p []byte) (n int, err error) {
	return b.buf.Read(p)
}

func (b *bufferState) Close() error {
	if _, ok := b.writer.(*cancellableGzipWriter); ok {
		return b.writer.(*cancellableGzipWriter).close()
	}
	return nil
}

// accept returns true if data is accepted by the buffer
func (b *bufferState) accept(data []byte) (bool, error) {
	_, err := b.writer.Write(data)
	overCapacity := errors.Is(err, errOverCapacity)
	bufLen := b.buf.Len()
	if overCapacity {
		bufLen += len(data)
	}
	if b.compressionAvailable && !b.compressionEnabled && bufLen > minCompressionLen {
		// switch over to a zip buffer.
		tmpBuf := bytes.NewBuffer(make([]byte, 0, b.bufferMaxLen+bufCapPadding))
		writer := gzip.NewWriter(tmpBuf)
		writer.Reset(tmpBuf)
		zipWriter := &cancellableGzipWriter{
			innerBuffer: tmpBuf,
			innerWriter: writer,
			// 8 bytes required for the zip footer.
			maxCapacity: b.bufferMaxLen - 8,
		}

		if b.bufferMaxLen == 0 {
			zipWriter.maxCapacity = 0
		}

		// the new data is so big, even with a zip writer, we are over the max limit.
		// abandon and return false, so we can send what is already in our buffer.
		if _, err2 := zipWriter.Write(b.buf.Bytes()); err2 != nil {
			return false, err2
		}
		b.writer = zipWriter
		b.buf = tmpBuf
		b.compressionEnabled = true
		// if the byte writer was over capacity, try to write the new entry in the zip writer:
		if overCapacity {
			if _, err2 := zipWriter.Write(data); err2 != nil {
				return false, err2
			}

		}
		return true, nil
	}
	if overCapacity {
		return false, nil
	}
	return true, err
}

type cancellableBytesWriter struct {
	innerWriter *bytes.Buffer
	maxCapacity uint
}

func (c *cancellableBytesWriter) Write(b []byte) (int, error) {
	if c.maxCapacity == 0 {
		return c.innerWriter.Write(b)
	}
	if c.innerWriter.Len()+len(b) > int(c.maxCapacity) {
		return 0, errOverCapacity
	}
	return c.innerWriter.Write(b)
}

type cancellableGzipWriter struct {
	innerBuffer *bytes.Buffer
	innerWriter *gzip.Writer
	maxCapacity uint
	len         int
}

func (c *cancellableGzipWriter) Write(b []byte) (int, error) {
	if c.maxCapacity == 0 {
		return c.innerWriter.Write(b)
	}
	c.len += len(b)
	// if we see that at a 50% compression rate, we'd be over max capacity, start flushing.
	if (c.len / 2) > int(c.maxCapacity) {
		// we flush so the length of the underlying buffer is accurate.
		if err := c.innerWriter.Flush(); err != nil {
			return 0, err
		}
	}
	// we find that the new content uncompressed, added to our buffer, would overflow our max capacity.
	if c.innerBuffer.Len()+len(b) > int(c.maxCapacity) {
		// so we create a copy of our content and add this new data, compressed, to check that it fits.
		copyBuf := bytes.NewBuffer(make([]byte, 0, c.maxCapacity+bufCapPadding))
		copyBuf.Write(c.innerBuffer.Bytes())
		writerCopy := gzip.NewWriter(copyBuf)
		writerCopy.Reset(copyBuf)
		if _, err := writerCopy.Write(b); err != nil {
			return 0, err
		}
		if err := writerCopy.Flush(); err != nil {
			return 0, err
		}
		// we find that even compressed, the data overflows.
		if copyBuf.Len() > int(c.maxCapacity) {
			return 0, errOverCapacity
		}
	}
	return c.innerWriter.Write(b)
}

func (c *cancellableGzipWriter) close() error {
	return c.innerWriter.Close()
}

// Composite index of a record.
type index struct {
	// Index in orig list (i.e. root parent index).
	resource int
	// Index in ScopeLogs/ScopeMetrics list (i.e. immediate parent index).
	library int
	// Index in Logs list (i.e. the log record index).
	record int
}

func (c *client) checkHecHealth() error {

	req, err := http.NewRequest("GET", c.healthCheckURL.String(), nil)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// Set the headers configured for the client
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) pushMetricsData(
	ctx context.Context,
	md pmetric.Metrics,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, bufState *bufferState) (err error) {
		localHeaders := map[string]string{}
		if md.ResourceMetrics().Len() != 0 {
			accessToken, found := md.ResourceMetrics().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
			if found {
				localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
			}
		}
		if err := bufState.Close(); err != nil {
			return err
		}
		return c.postEvents(ctx, bufState, localHeaders, bufState.compressionEnabled, bufState.buf.Len())
	}

	return c.pushMetricsDataInBatches(ctx, md, send)
}

func (c *client) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, bufState *bufferState) (err error) {
		localHeaders := map[string]string{}
		if td.ResourceSpans().Len() != 0 {
			accessToken, found := td.ResourceSpans().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
			if found {
				localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
			}
		}
		if err := bufState.Close(); err != nil {
			return err
		}
		return c.postEvents(ctx, bufState, localHeaders, bufState.compressionEnabled, bufState.buf.Len())
	}

	return c.pushTracesDataInBatches(ctx, td, send)
}

func (c *client) pushLogData(ctx context.Context, ld plog.Logs) error {
	c.wg.Add(1)
	defer c.wg.Done()

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, bufState *bufferState, headers map[string]string) (err error) {
		localHeaders := headers
		if ld.ResourceLogs().Len() != 0 {
			accessToken, found := ld.ResourceLogs().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
			if found {
				localHeaders = map[string]string{}
				for k, v := range headers {
					localHeaders[k] = v
				}
				localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
			}
		}
		if err := bufState.Close(); err != nil {
			return err
		}
		return c.postEvents(ctx, bufState, localHeaders, bufState.compressionEnabled, bufState.buf.Len())
	}

	return c.pushLogDataInBatches(ctx, ld, send)
}

// A guesstimated value > length of bytes of a single event.
// Added to buffer capacity so that buffer is likely to grow by reslicing when buf.Len() > bufCap.
const bufCapPadding = uint(4096)
const libraryHeaderName = "X-Splunk-Instrumentation-Library"
const profilingLibraryName = "otel.profiling"

var profilingHeaders = map[string]string{
	libraryHeaderName: profilingLibraryName,
}

func isProfilingData(sl plog.ScopeLogs) bool {
	return sl.Scope().Name() == profilingLibraryName
}

func makeBlankBufferState(bufCap uint, compressionAvailable bool) *bufferState {
	// Buffer of JSON encoded Splunk events, last record is expected to overflow bufCap, hence the padding
	buf := bytes.NewBuffer(make([]byte, 0, bufCap+bufCapPadding))

	return &bufferState{
		compressionAvailable: compressionAvailable,
		compressionEnabled:   false,
		writer:               &cancellableBytesWriter{innerWriter: buf, maxCapacity: bufCap},
		buf:                  buf,
		bufferMaxLen:         bufCap,
		bufFront:             nil, // Index of the log record of the first unsent event in buffer.
		resource:             0,   // Index of currently processed Resource
		library:              0,   // Index of currently processed Library
	}
}

// pushLogDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthLogs.
// ld log records are parsed to Splunk events.
// The input data may contain both logs and profiling data.
// They are batched separately and sent with different HTTP headers
func (c *client) pushLogDataInBatches(ctx context.Context, ld plog.Logs, send func(context.Context, *bufferState, map[string]string) error) error {
	var bufState = makeBlankBufferState(c.config.MaxContentLengthLogs, !c.config.DisableCompression)
	var profilingBufState = makeBlankBufferState(c.config.MaxContentLengthLogs, !c.config.DisableCompression)
	var permanentErrors []error

	var rls = ld.ResourceLogs()
	var droppedProfilingDataRecords, droppedLogRecords int
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			var err error
			var newPermanentErrors []error

			if isProfilingData(ills.At(j)) {
				if !c.config.ProfilingDataEnabled {
					droppedProfilingDataRecords += ills.At(j).LogRecords().Len()
					continue
				}
				profilingBufState.resource, profilingBufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, profilingBufState, profilingHeaders, send)
			} else {
				if !c.config.LogDataEnabled {
					droppedLogRecords += ills.At(j).LogRecords().Len()
					continue
				}
				bufState.resource, bufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, bufState, nil, send)
			}

			if err != nil {
				return consumererror.NewLogs(err, c.subLogs(ld, bufState.bufFront, profilingBufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	if droppedProfilingDataRecords != 0 {
		c.logger.Debug("Profiling data is not allowed", zap.Int("dropped_records", droppedProfilingDataRecords))
	}
	if droppedLogRecords != 0 {
		c.logger.Debug("Log data is not allowed", zap.Int("dropped_records", droppedLogRecords))
	}

	// There's some leftover unsent non-profiling data
	if bufState.buf.Len() > 0 {

		if err := send(ctx, bufState, nil); err != nil {
			return consumererror.NewLogs(err, c.subLogs(ld, bufState.bufFront, profilingBufState.bufFront))
		}
	}

	// There's some leftover unsent profiling data
	if profilingBufState.buf.Len() > 0 {
		if err := send(ctx, profilingBufState, profilingHeaders); err != nil {
			// Non-profiling bufFront is set to nil because all non-profiling data was flushed successfully above.
			return consumererror.NewLogs(err, c.subLogs(ld, nil, profilingBufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) pushLogRecords(ctx context.Context, lds plog.ResourceLogsSlice, state *bufferState, headers map[string]string, send func(context.Context, *bufferState, map[string]string) error) (permanentErrors []error, sendingError error) {
	res := lds.At(state.resource)
	logs := res.ScopeLogs().At(state.library).LogRecords()

	for k := 0; k < logs.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}

		// Parsing log record to Splunk event.
		event := mapLogRecordToSplunkEvent(res.Resource(), logs.At(k), c.config, c.logger)
		// JSON encoding event and writing to buffer.
		b, err := jsoniter.Marshal(event)
		if err != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped log event: %v, error: %w", event, err)))
			continue
		}

		// Continue adding events to buffer up to capacity.
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.buf.Len() > 0 {
			if err = send(ctx, state, headers); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped log event error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}
		if state.buf.Len() > 0 {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

func (c *client) pushMetricsRecords(ctx context.Context, mds pmetric.ResourceMetricsSlice, state *bufferState, send func(context.Context, *bufferState) error) (permanentErrors []error, sendingError error) {
	res := mds.At(state.resource)
	metrics := res.ScopeMetrics().At(state.library).Metrics()

	for k := 0; k < metrics.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}

		// Parsing metric record to Splunk event.
		events := mapMetricToSplunkEvent(res.Resource(), metrics.At(k), c.config, c.logger)
		buf := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthMetrics))
		for _, event := range events {
			// JSON encoding event and writing to buffer.
			b, err := jsoniter.Marshal(event)
			if err != nil {
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped metric event: %v, error: %w", event, err)))
				continue
			}
			buf.Write(b)
		}

		// Continue adding events to buffer up to capacity.
		b := buf.Bytes()
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.buf.Len() > 0 {
			if err := send(ctx, state); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped metric event: error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}

		if state.buf.Len() > 0 {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

func (c *client) pushTracesData(ctx context.Context, tds ptrace.ResourceSpansSlice, state *bufferState, send func(context.Context, *bufferState) error) (permanentErrors []error, sendingError error) {
	res := tds.At(state.resource)
	spans := res.ScopeSpans().At(state.library).Spans()

	for k := 0; k < spans.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}

		// Parsing span record to Splunk event.
		event := mapSpanToSplunkEvent(res.Resource(), spans.At(k), c.config, c.logger)
		// JSON encoding event and writing to buffer.
		b, err := jsoniter.Marshal(event)
		if err != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped span events: %v, error: %w", event, err)))
			continue
		}

		// Continue adding events to buffer up to capacity.
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.buf.Len() > 0 {
			if err = send(ctx, state); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped trace event error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}

		if state.buf.Len() > 0 {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

// pushMetricsDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// md metrics are parsed to Splunk events.
func (c *client) pushMetricsDataInBatches(ctx context.Context, md pmetric.Metrics, send func(context.Context, *bufferState) error) error {
	var bufState = makeBlankBufferState(c.config.MaxContentLengthMetrics, !c.config.DisableCompression)
	var permanentErrors []error

	var rms = md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			var err error
			var newPermanentErrors []error

			bufState.resource, bufState.library = i, j
			newPermanentErrors, err = c.pushMetricsRecords(ctx, rms, bufState, send)

			if err != nil {
				return consumererror.NewMetrics(err, subMetrics(md, bufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	// There's some leftover unsent metrics
	if bufState.buf.Len() > 0 {
		if err := send(ctx, bufState); err != nil {
			return consumererror.NewMetrics(err, subMetrics(md, bufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

// pushTracesDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// td traces are parsed to Splunk events.
func (c *client) pushTracesDataInBatches(ctx context.Context, td ptrace.Traces, send func(context.Context, *bufferState) error) error {
	bufState := makeBlankBufferState(c.config.MaxContentLengthTraces, !c.config.DisableCompression)
	var permanentErrors []error

	var rts = td.ResourceSpans()
	for i := 0; i < rts.Len(); i++ {
		ilts := rts.At(i).ScopeSpans()
		for j := 0; j < ilts.Len(); j++ {
			var err error
			var newPermanentErrors []error

			bufState.resource, bufState.library = i, j
			newPermanentErrors, err = c.pushTracesData(ctx, rts, bufState, send)

			if err != nil {
				return consumererror.NewTraces(err, subTraces(td, bufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	// There's some leftover unsent traces
	if bufState.buf.Len() > 0 {
		if err := send(ctx, bufState); err != nil {
			return consumererror.NewTraces(err, subTraces(td, bufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) postEvents(ctx context.Context, events io.Reader, headers map[string]string, compressed bool, contentLength int) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), events)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.ContentLength = int64(contentLength)

	// Set the headers configured for the client
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Set extra headers passed by the caller
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	_, errCopy := io.Copy(io.Discard, resp.Body)
	return multierr.Combine(err, errCopy)
}

// subLogs returns a subset of `ld` starting from `profilingBufFront` for profiling data
// plus starting from `bufFront` for non-profiling data. Both can be nil, in which case they are ignored
func (c *client) subLogs(ld plog.Logs, bufFront *index, profilingBufFront *index) plog.Logs {
	subset := plog.NewLogs()
	if c.config.LogDataEnabled {
		subLogsByType(ld, bufFront, subset, false)
	}
	if c.config.ProfilingDataEnabled {
		subLogsByType(ld, profilingBufFront, subset, true)
	}

	return subset
}

// subMetrics returns a subset of `md`starting from `bufFront`. It can be nil, in which case it is ignored
func subMetrics(md pmetric.Metrics, bufFront *index) pmetric.Metrics {
	subset := pmetric.NewMetrics()
	subMetricsByType(md, bufFront, subset)

	return subset
}

// subTraces returns a subset of `td`starting from `bufFront`. It can be nil, in which case it is ignored
func subTraces(td ptrace.Traces, bufFront *index) ptrace.Traces {
	subset := ptrace.NewTraces()
	subTracesByType(td, bufFront, subset)

	return subset
}

func subLogsByType(src plog.Logs, from *index, dst plog.Logs, profiling bool) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceLogs()
	resourcesSub := dst.ResourceLogs()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeLogs()
		librariesSub := newSub.ScopeLogs()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			// Only copy profiling data if requested. If not requested, only copy non-profiling data
			if profiling != isProfilingData(lib) {
				continue
			}

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			logs := lib.LogRecords()
			logsSub := newLibSub.LogRecords()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < logs.Len(); k++ { //revive:disable-line:var-naming
				logs.At(k).CopyTo(logsSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func subMetricsByType(src pmetric.Metrics, from *index, dst pmetric.Metrics) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceMetrics()
	resourcesSub := dst.ResourceMetrics()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeMetrics()
		librariesSub := newSub.ScopeMetrics()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			metrics := lib.Metrics()
			metricsSub := newLibSub.Metrics()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < metrics.Len(); k++ { //revive:disable-line:var-naming
				metrics.At(k).CopyTo(metricsSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func subTracesByType(src ptrace.Traces, from *index, dst ptrace.Traces) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceSpans()
	resourcesSub := dst.ResourceSpans()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeSpans()
		librariesSub := newSub.ScopeSpans()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			traces := lib.Spans()
			tracesSub := newLibSub.Spans()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < traces.Len(); k++ { //revive:disable-line:var-naming
				traces.At(k).CopyTo(tracesSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func (c *client) stop(context.Context) error {
	c.wg.Wait()
	return nil
}

func (c *client) start(context.Context, component.Host) (err error) {
	return nil
}
