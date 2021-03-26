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
	"go.opentelemetry.io/collector/consumer/pdata"
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

	body, compressed, err := encodeBody(&c.zippers, splunkDataPoints, c.config.DisableCompression)
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

	// Splunk accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return err
	}

	return nil
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

func (c *client) pushLogData(ctx context.Context, ld pdata.Logs) (err error) {
	c.wg.Add(1)
	defer c.wg.Done()

	gzipWriter := c.zippers.Get().(*gzip.Writer)
	defer c.zippers.Put(gzipWriter)

	gzipBuffer := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs))
	gzipWriter.Reset(gzipBuffer)

	defer gzipWriter.Close()

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, buf *bytes.Buffer) (err error) {
		shouldCompress := buf.Len() >= minCompressionLen && !c.config.DisableCompression

		if shouldCompress {
			gzipBuffer.Reset()
			gzipWriter.Reset(gzipBuffer)

			if _, err = io.Copy(gzipWriter, buf); err != nil {
				return fmt.Errorf("failed copying buffer to gzip writer: %v", err)
			}

			if err = gzipWriter.Flush(); err != nil {
				return fmt.Errorf("failed flushing compressed data to gzip writer: %v", err)
			}

			return c.postEvents(ctx, gzipBuffer, shouldCompress)
		}

		return c.postEvents(ctx, buf, shouldCompress)
	}

	return c.pushLogDataParts(ctx, ld, send)
}

// pushLogDataParts partitions log data then pushes the parts.
func (c *client) pushLogDataParts(ctx context.Context, ld pdata.Logs, send func(context.Context, *bytes.Buffer) error) (err error) {
	// max is the maximum number of bytes allowed in logs buffer.
	// pad4096 pads max by 4096 to prevent reallocation in logs buffer prior to truncation. It is assumed that a single log is likely less than 4096 bytes.
	var max, pad4096 = c.config.MaxContentLengthLogs, uint(4096)
	// Buffer of a part of log data.
	var logsBuf = bytes.NewBuffer(make([]byte, 0, max+pad4096))
	var encoder = json.NewEncoder(logsBuf)
	// Number of bytes below max of logs in logsBuf.
	var length int
	// Temp storage for log over max in logsBuf.
	var overMaxBuf = bytes.NewBuffer(make([]byte, 0, pad4096))
	// Index of the first log in a logs part.
	var log0Index *logIndex
	var permanentErrors []error
	var rls = ld.ResourceLogs()

	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				if log0Index == nil {
					log0Index = &logIndex{resource: i, library: j, record: k}
				}

				event := mapLogRecordToSplunkEvent(logs.At(k), c.config, c.logger)
				if err = encoder.Encode(event); err != nil {
					permanentErrors = append(permanentErrors, consumererror.Permanent(fmt.Errorf("dropped log event: %v, error: %v", event, err)))
					continue
				}
				logsBuf.WriteString("\r\n\r\n")

				// Consistent with ContentLength in http.Request, MaxContentLengthLogs value of 0 indicates length unknown (i.e. unbound).
				if max == 0 || logsBuf.Len() <= int(max) {
					length = logsBuf.Len()
					continue
				}

				overMaxBuf.Reset()
				if max > 0 {
					if overLength := logsBuf.Len() - length; overLength <= int(max) {
						overMaxBuf.Write(logsBuf.Bytes()[length:logsBuf.Len()])
					} else {
						permanentErrors = append(permanentErrors, consumererror.Permanent(fmt.Errorf("dropped log event: %s, error: event size %d bytes larger than configured max content length %d bytes", string(logsBuf.Bytes()[length:logsBuf.Len()]), overLength, max)))
					}
				}

				logsBuf.Truncate(length)
				if logsBuf.Len() > 0 {
					if err = send(ctx, logsBuf); err != nil {
						return consumererror.PartialLogsError(err, *subLogs(&ld, log0Index))
					}
				}
				logsBuf.Reset()
				overMaxBuf.WriteTo(logsBuf)

				log0Index, length = nil, logsBuf.Len()
			}
		}
	}

	if logsBuf.Len() > 0 {
		if err = send(ctx, logsBuf); err != nil {
			return consumererror.PartialLogsError(err, *subLogs(&ld, log0Index))
		}
	}

	return consumererror.CombineErrors(permanentErrors)
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

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// Splunk accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return err
	}
	return nil
}

// subLogs returns a subset of `ld` starting from index `start` to the end.
func subLogs(ld *pdata.Logs, start *logIndex) *pdata.Logs {
	if ld == nil || start == nil || start.zero() {
		return ld
	}

	logs := pdata.NewLogs()

	rl, rl2 := ld.ResourceLogs(), logs.ResourceLogs()

	for r := start.resource; r < rl.Len(); r++ {
		rl2.Append(pdata.NewResourceLogs())
		rl.At(r).Resource().CopyTo(rl2.At(r - start.resource).Resource())

		il, il2 := rl.At(r).InstrumentationLibraryLogs(), rl2.At(r-start.resource).InstrumentationLibraryLogs()

		i := 0
		if r == start.resource {
			i = start.library
		}
		for i2 := 0; i < il.Len(); i++ {
			il2.Append(pdata.NewInstrumentationLibraryLogs())
			il.At(i).InstrumentationLibrary().CopyTo(il2.At(i2).InstrumentationLibrary())

			lr, lr2 := il.At(i).Logs(), il2.At(i2).Logs()
			i2++

			l := 0
			if r == start.resource && i == start.library {
				l = start.record
			}
			for l2 := 0; l < lr.Len(); l++ {
				lr2.Append(pdata.NewLogRecord())
				lr.At(l).CopyTo(lr2.At(l2))
				l2++
			}
		}
	}

	return &logs
}

func encodeBodyEvents(zippers *sync.Pool, evs []*splunk.Event, disableCompression bool) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range evs {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
		buf.WriteString("\r\n\r\n")
	}
	return getReader(zippers, buf, disableCompression)
}

func encodeBody(zippers *sync.Pool, dps []*splunk.Event, disableCompression bool) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range dps {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
		buf.WriteString("\r\n\r\n")
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

func (c *client) stop(context context.Context) error {
	c.wg.Wait()
	return nil
}

func (c *client) start(context.Context, component.Host) (err error) {
	return nil
}
