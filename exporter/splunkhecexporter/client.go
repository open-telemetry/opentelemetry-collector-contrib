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

package splunkhecexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// client sends the data to the splunk backend.
type client struct {
	config  *Config
	url     *url.URL
	client  *http.Client
	logger  *zap.Logger
	zippers sync.Pool
	wg      sync.WaitGroup
	headers map[string]string
}

// bufferState encapsulates intermediate buffer state when pushing log data
type bufferState struct {
	buf      *bytes.Buffer
	encoder  *json.Encoder
	tmpBuf   *bytes.Buffer
	bufFront *logIndex
	bufLen   int
	resource int
	library  int
}

// Minimum number of bytes to compress. 1500 is the MTU of an ethernet frame.
const minCompressionLen = 1500

func (c *client) pushMetricsData(
	ctx context.Context,
	md pdata.Metrics,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	splunkDataPoints, _ := metricDataToSplunk(c.logger, md, c.config)
	if len(splunkDataPoints) == 0 {
		return nil
	}

	body, compressed, err := encodeBodyEvents(&c.zippers, splunkDataPoints, c.config.DisableCompression)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), body)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	return splunk.HandleHTTPCode(resp)
}

func (c *client) pushTraceData(
	ctx context.Context,
	td pdata.Traces,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	splunkEvents, _ := traceDataToSplunk(c.logger, td, c.config)
	if len(splunkEvents) == 0 {
		return nil
	}

	return c.sendSplunkEvents(ctx, splunkEvents)
}

func (c *client) sendSplunkEvents(ctx context.Context, splunkEvents []*splunk.Event) error {
	body, compressed, err := encodeBodyEvents(&c.zippers, splunkEvents, c.config.DisableCompression)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return c.postEvents(ctx, body, nil, compressed)
}

func (c *client) pushLogData(ctx context.Context, ld pdata.Logs) error {
	c.wg.Add(1)
	defer c.wg.Done()

	gzipWriter := c.zippers.Get().(*gzip.Writer)
	defer c.zippers.Put(gzipWriter)

	gzipBuffer := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs))
	gzipWriter.Reset(gzipBuffer)

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, buf *bytes.Buffer, headers map[string]string) (err error) {
		shouldCompress := buf.Len() >= minCompressionLen && !c.config.DisableCompression

		if shouldCompress {
			gzipBuffer.Reset()
			gzipWriter.Reset(gzipBuffer)

			if _, err = io.Copy(gzipWriter, buf); err != nil {
				return fmt.Errorf("failed copying buffer to gzip writer: %v", err)
			}

			if err = gzipWriter.Close(); err != nil {
				return fmt.Errorf("failed flushing compressed data to gzip writer: %v", err)
			}

			return c.postEvents(ctx, gzipBuffer, headers, shouldCompress)
		}

		return c.postEvents(ctx, buf, headers, shouldCompress)
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

func isProfilingData(ill pdata.InstrumentationLibraryLogs) bool {
	return ill.InstrumentationLibrary().Name() == profilingLibraryName
}

func makeBlankBufferState(bufCap uint) bufferState {
	// Buffer of JSON encoded Splunk events, last record is expected to overflow bufCap, hence the padding
	var buf = bytes.NewBuffer(make([]byte, 0, bufCap+bufCapPadding))
	var encoder = json.NewEncoder(buf)

	// Buffer for overflown records that do not fit in the main buffer
	var tmpBuf = bytes.NewBuffer(make([]byte, 0, bufCapPadding))

	return bufferState{
		buf:      buf,
		encoder:  encoder,
		tmpBuf:   tmpBuf,
		bufFront: nil, // Index of the log record of the first unsent event in buffer.
		bufLen:   0,   // Length of data in buffer excluding the last record if it overflows bufCap
		resource: 0,   // Index of currently processed Resource
		library:  0,   // Index of currently processed Library
	}
}

// pushLogDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthLogs.
// ld log records are parsed to Splunk events.
// The input data may contain both logs and profiling data.
// They are batched separately and sent with different HTTP headers
func (c *client) pushLogDataInBatches(ctx context.Context, ld pdata.Logs, send func(context.Context, *bytes.Buffer, map[string]string) error) error {
	var bufState = makeBlankBufferState(c.config.MaxContentLengthLogs)
	var profilingBufState = makeBlankBufferState(c.config.MaxContentLengthLogs)
	var permanentErrors []error

	var rls = ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			var err error
			var newPermanentErrors []error

			if isProfilingData(ills.At(j)) {
				profilingBufState.resource, profilingBufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, &profilingBufState, profilingHeaders, send)
			} else {
				bufState.resource, bufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, &bufState, nil, send)
			}

			if err != nil {
				return consumererror.NewLogs(err, *subLogs(&ld, bufState.bufFront, profilingBufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	// There's some leftover unsent non-profiling data
	if bufState.buf.Len() > 0 {
		if err := send(ctx, bufState.buf, nil); err != nil {
			return consumererror.NewLogs(err, *subLogs(&ld, bufState.bufFront, profilingBufState.bufFront))
		}
	}

	// There's some leftover unsent profiling data
	if profilingBufState.buf.Len() > 0 {
		if err := send(ctx, profilingBufState.buf, profilingHeaders); err != nil {
			// Non-profiling bufFront is set to nil because all non-profiling data was flushed successfully above.
			return consumererror.NewLogs(err, *subLogs(&ld, nil, profilingBufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) pushLogRecords(ctx context.Context, lds pdata.ResourceLogsSlice, state *bufferState, headers map[string]string, send func(context.Context, *bytes.Buffer, map[string]string) error) (permanentErrors []error, sendingError error) {
	res := lds.At(state.resource)
	logs := res.InstrumentationLibraryLogs().At(state.library).Logs()
	bufCap := int(c.config.MaxContentLengthLogs)

	for k := 0; k < logs.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &logIndex{resource: state.resource, library: state.library, record: k}
		}

		// Parsing log record to Splunk event.
		event := mapLogRecordToSplunkEvent(res.Resource(), logs.At(k), c.config, c.logger)
		// JSON encoding event and writing to buffer.
		if err := state.encoder.Encode(event); err != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped log event: %v, error: %v", event, err)))
			continue
		}

		// Continue adding events to buffer up to capacity.
		// 0 capacity is interpreted as unknown/unbound consistent with ContentLength in http.Request.
		if state.buf.Len() <= bufCap || bufCap == 0 {
			// Tracking length of event bytes below capacity in buffer.
			state.bufLen = state.buf.Len()
			continue
		}

		state.tmpBuf.Reset()
		// Storing event bytes over capacity in buffer before truncating.
		if bufCap > 0 {
			if over := state.buf.Len() - state.bufLen; over <= bufCap {
				state.tmpBuf.Write(state.buf.Bytes()[state.bufLen:state.buf.Len()])
			} else {
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(
					fmt.Errorf("dropped log event: %s, error: event size %d bytes larger than configured max content length %d bytes", string(state.buf.Bytes()[state.bufLen:state.buf.Len()]), over, bufCap)))
			}
		}

		// Truncating buffer at tracked length below capacity and sending.
		state.buf.Truncate(state.bufLen)
		if state.buf.Len() > 0 {
			if err := send(ctx, state.buf, headers); err != nil {
				return permanentErrors, err
			}
		}
		state.buf.Reset()

		// Writing truncated bytes back to buffer.
		state.tmpBuf.WriteTo(state.buf)

		if state.buf.Len() > 0 {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &logIndex{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

		state.bufLen = state.buf.Len()
	}

	return permanentErrors, nil
}

func (c *client) postEvents(ctx context.Context, events io.Reader, headers map[string]string, compressed bool) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), events)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

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

	io.Copy(ioutil.Discard, resp.Body)

	return err
}

// subLogs returns a subset of `ld` starting from `profilingBufFront` for profiling data
// plus starting from `bufFront` for non-profiling data. Both can be nil, in which case they are ignored
func subLogs(ld *pdata.Logs, bufFront *logIndex, profilingBufFront *logIndex) *pdata.Logs {
	if ld == nil {
		return ld
	}

	subset := pdata.NewLogs()
	subLogsByType(ld, bufFront, &subset, false)
	subLogsByType(ld, profilingBufFront, &subset, true)

	return &subset
}

func subLogsByType(src *pdata.Logs, from *logIndex, dst *pdata.Logs, profiling bool) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceLogs()
	resourcesSub := dst.ResourceLogs()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).InstrumentationLibraryLogs()
		librariesSub := newSub.InstrumentationLibraryLogs()

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
			lib.InstrumentationLibrary().CopyTo(newLibSub.InstrumentationLibrary())

			logs := lib.Logs()
			logsSub := newLibSub.Logs()
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

func encodeBodyEvents(zippers *sync.Pool, evs []*splunk.Event, disableCompression bool) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range evs {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
	}
	return getReader(zippers, buf, disableCompression)
}

// avoid attempting to compress things that fit into a single ethernet frame
func getReader(zippers *sync.Pool, b *bytes.Buffer, disableCompression bool) (io.Reader, bool, error) {
	var err error
	if !disableCompression && b.Len() > minCompressionLen {
		buf := new(bytes.Buffer)
		w := zippers.Get().(*gzip.Writer)
		defer zippers.Put(w)
		w.Reset(buf)
		_, err = w.Write(b.Bytes())
		if err == nil {
			err = w.Close()
			if err == nil {
				return buf, true, nil
			}
		}
	}
	return b, false, err
}

func (c *client) stop(context.Context) error {
	c.wg.Wait()
	return nil
}

func (c *client) start(context.Context, component.Host) (err error) {
	return nil
}
