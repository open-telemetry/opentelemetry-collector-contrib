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
		return consumererror.Permanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), body)
	if err != nil {
		return consumererror.Permanent(err)
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
		return consumererror.Permanent(err)
	}

	return c.postEvents(ctx, body, compressed)
}

func (c *client) pushLogData(ctx context.Context, ld pdata.Logs) error {
	c.wg.Add(1)
	defer c.wg.Done()

	gzipWriter := c.zippers.Get().(*gzip.Writer)
	defer c.zippers.Put(gzipWriter)

	gzipBuffer := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs))
	gzipWriter.Reset(gzipBuffer)

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, buf *bytes.Buffer) (err error) {
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

			return c.postEvents(ctx, gzipBuffer, shouldCompress)
		}

		return c.postEvents(ctx, buf, shouldCompress)
	}

	return c.pushLogDataInBatches(ctx, ld, send)
}

// pushLogDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthLogs.
// ld log records are parsed to Splunk events.
func (c *client) pushLogDataInBatches(ctx context.Context, ld pdata.Logs, send func(context.Context, *bytes.Buffer) error) error {
	// Length of retained bytes in buffer after truncation.
	var bufLen int
	// Buffer capacity.
	var bufCap = c.config.MaxContentLengthLogs
	// A guesstimated value > length of bytes of a single event.
	// Added to buffer capacity so that buffer is likely to grow by reslicing when buf.Len() > bufCap.
	const bufCapPadding = uint(4096)

	// Buffer of JSON encoded Splunk events.
	// Expected to grow more than bufCap then truncated to bufLen.
	var buf = bytes.NewBuffer(make([]byte, 0, bufCap+bufCapPadding))
	var encoder = json.NewEncoder(buf)

	var tmpBuf = bytes.NewBuffer(make([]byte, 0, bufCapPadding))

	// Index of the log record of the first event in buffer.
	var bufFront *logIndex

	var permanentErrors []error

	var rls = ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		res := rls.At(i).Resource()
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				if bufFront == nil {
					bufFront = &logIndex{resource: i, library: j, record: k}
				}

				// Parsing log record to Splunk event.
				event := mapLogRecordToSplunkEvent(res, logs.At(k), c.config, c.logger)
				// JSON encoding event and writing to buffer.
				if err := encoder.Encode(event); err != nil {
					permanentErrors = append(permanentErrors, consumererror.Permanent(fmt.Errorf("dropped log event: %v, error: %v", event, err)))
					continue
				}

				// Continue adding events to buffer up to capacity.
				// 0 capacity is interpreted as unknown/unbound consistent with ContentLength in http.Request.
				if buf.Len() <= int(bufCap) || bufCap == 0 {
					// Tracking length of event bytes below capacity in buffer.
					bufLen = buf.Len()
					continue
				}

				tmpBuf.Reset()
				// Storing event bytes over capacity in buffer before truncating.
				if bufCap > 0 {
					if over := buf.Len() - bufLen; over <= int(bufCap) {
						tmpBuf.Write(buf.Bytes()[bufLen:buf.Len()])
					} else {
						permanentErrors = append(permanentErrors, consumererror.Permanent(
							fmt.Errorf("dropped log event: %s, error: event size %d bytes larger than configured max content length %d bytes", string(buf.Bytes()[bufLen:buf.Len()]), over, bufCap)))
					}
				}

				// Truncating buffer at tracked length below capacity and sending.
				buf.Truncate(bufLen)
				if buf.Len() > 0 {
					if err := send(ctx, buf); err != nil {
						return consumererror.NewLogs(err, *subLogs(&ld, bufFront))
					}
				}
				buf.Reset()

				// Writing truncated bytes back to buffer.
				tmpBuf.WriteTo(buf)

				bufFront, bufLen = nil, buf.Len()
			}
		}
	}

	if buf.Len() > 0 {
		if err := send(ctx, buf); err != nil {
			return consumererror.NewLogs(err, *subLogs(&ld, bufFront))
		}
	}

	return consumererror.Combine(permanentErrors)
}

func (c *client) postEvents(ctx context.Context, events io.Reader, compressed bool) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), events)
	if err != nil {
		return consumererror.Permanent(err)
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
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)

	io.Copy(ioutil.Discard, resp.Body)

	return err
}

// subLogs returns a subset of `ld` starting from index `from` to the end.
func subLogs(ld *pdata.Logs, from *logIndex) *pdata.Logs {
	if ld == nil || from == nil || from.zero() {
		return ld
	}

	subset := pdata.NewLogs()

	resources := ld.ResourceLogs()
	resourcesSub := subset.ResourceLogs()

	for i := from.resource; i < resources.Len(); i++ {
		resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(resourcesSub.At(i - from.resource).Resource())

		libraries := resources.At(i).InstrumentationLibraryLogs()
		librariesSub := resourcesSub.At(i - from.resource).InstrumentationLibraryLogs()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			librariesSub.AppendEmpty()
			libraries.At(j).InstrumentationLibrary().CopyTo(librariesSub.At(jSub).InstrumentationLibrary())

			logs := libraries.At(j).Logs()
			logsSub := librariesSub.At(jSub).Logs()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < logs.Len(); k++ { //revive:disable-line:var-naming
				logsSub.AppendEmpty()
				logs.At(k).CopyTo(logsSub.At(kSub))
				kSub++
			}
		}
	}

	return &subset
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
