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

func (c *client) pushMetricsData(
	ctx context.Context,
	md pdata.Metrics,
) (droppedTimeSeries int, err error) {
	c.wg.Add(1)
	defer c.wg.Done()

	splunkDataPoints, numDroppedTimeseries := metricDataToSplunk(c.logger, md, c.config)
	if len(splunkDataPoints) == 0 {
		return numDroppedTimeseries, nil
	}

	body, compressed, err := encodeBody(&c.zippers, splunkDataPoints, c.config.DisableCompression, c.config.MinContentLengthCompression)
	if err != nil {
		return numMetricPoint(md), consumererror.Permanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), body)
	if err != nil {
		return numMetricPoint(md), consumererror.Permanent(err)
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return numMetricPoint(md), err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// Splunk accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return numMetricPoint(md), err
	}

	return numDroppedTimeseries, nil
}

func (c *client) pushTraceData(
	ctx context.Context,
	td pdata.Traces,
) (droppedSpans int, err error) {
	c.wg.Add(1)
	defer c.wg.Done()

	splunkEvents, numDroppedSpans := traceDataToSplunk(c.logger, td, c.config)
	if len(splunkEvents) == 0 {
		return numDroppedSpans, nil
	}

	err = c.sendSplunkEvents(ctx, splunkEvents)
	if err != nil {
		return td.SpanCount(), err
	}

	return numDroppedSpans, nil
}

func (c *client) sendSplunkEvents(ctx context.Context, splunkEvents []*splunk.Event) error {
	body, compressed, err := encodeBodyEvents(&c.zippers, splunkEvents, c.config.DisableCompression, c.config.MinContentLengthCompression)
	if err != nil {
		return consumererror.Permanent(err)
	}

	return c.postEvents(ctx, body, compressed)
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

func subLogs(ld *pdata.Logs, logIdx *logIndex) *pdata.Logs {
	if logIdx.zero() {
		return ld
	}
	clone := ld.Clone().InternalRep()

	subset := *clone.Orig
	subset = subset[logIdx.resourceIdx:]
	subset[0].InstrumentationLibraryLogs = subset[0].InstrumentationLibraryLogs[logIdx.libraryIdx:]
	subset[0].InstrumentationLibraryLogs[0].Logs = subset[0].InstrumentationLibraryLogs[0].Logs[logIdx.recordIdx:]

	clone.Orig = &subset
	logs := pdata.LogsFromInternalRep(clone)
	return &logs
}

func (c *client) sentLogBatch(ctx context.Context, ld pdata.Logs, send func(context.Context, *bytes.Buffer) error) (numDroppedLogs int, err error) {
	var submax int
	var index *logIndex
	var permanentErrors []error

	// Provide 5000 overflow because it overruns the max content length then trims it block. Hopefully will prevent extra allocation.
	batch := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs+5_000))
	encoder := json.NewEncoder(batch)

	overflow := bytes.NewBuffer(make([]byte, 0, 5000))

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				if index == nil {
					index = &logIndex{resourceIdx: i, libraryIdx: j, recordIdx: k}
				}

				event := mapLogRecordToSplunkEvent(logs.At(k), c.config, c.logger)
				if err = encoder.Encode(event); err != nil {
					permanentErrors = append(permanentErrors, consumererror.Permanent(fmt.Errorf("dropped log event: %v, error: %v", event, err)))
					continue
				}
				batch.WriteString("\r\n\r\n")

				// Consistent with ContentLength in http.Request, MaxContentLengthLogs value of 0 indicates length unknown (i.e. unbound).
				if c.config.MaxContentLengthLogs == 0 || batch.Len() <= int(c.config.MaxContentLengthLogs) {
					submax = batch.Len()
					continue
				}

				overflow.Reset()
				if c.config.MaxContentLengthLogs > 0 {
					if over := batch.Len() - submax; over <= int(c.config.MaxContentLengthLogs) {
						overflow.Write(batch.Bytes()[submax:batch.Len()])
					} else {
						permanentErrors = append(permanentErrors, consumererror.Permanent(fmt.Errorf("dropped log event: %s, error: event size %d bytes larger than configured max content length %d bytes", string(batch.Bytes()[submax:batch.Len()]), over, c.config.MaxContentLengthLogs)))
					}
				}

				batch.Truncate(submax)
				if batch.Len() > 0 {
					if err = send(ctx, batch); err != nil {
						dropped := subLogs(&ld, index)
						return dropped.LogRecordCount(), consumererror.PartialLogsError(err, *dropped)
					}
				}
				batch.Reset()
				overflow.WriteTo(batch)

				index, submax = nil, batch.Len()
			}
		}
	}

	if batch.Len() > 0 {
		if err = send(ctx, batch); err != nil {
			dropped := subLogs(&ld, index)
			return dropped.LogRecordCount(), consumererror.PartialLogsError(err, *dropped)
		}
	}

	return len(permanentErrors), consumererror.CombineErrors(permanentErrors)
}

func (c *client) pushLogData(ctx context.Context, ld pdata.Logs) (numDroppedLogs int, err error) {
	c.wg.Add(1)
	defer c.wg.Done()

	gzipWriter := c.zippers.Get().(*gzip.Writer)
	defer c.zippers.Put(gzipWriter)

	gzipBuffer := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs))
	gzipWriter.Reset(gzipBuffer)

	defer gzipWriter.Close()

	// Callback when each batch is to be sent.
	send := func(ctx context.Context, buf *bytes.Buffer) (err error) {
		compression := buf.Len() >= int(c.config.MinContentLengthCompression) && !c.config.DisableCompression

		if compression {
			gzipBuffer.Reset()
			gzipWriter.Reset(gzipBuffer)

			if _, err = io.Copy(gzipWriter, buf); err != nil {
				return fmt.Errorf("failed copying buffer to gzip writer: %v", err)
			}

			if err = gzipWriter.Flush(); err != nil {
				return fmt.Errorf("failed flushing compressed data to gzip writer: %v", err)
			}

			return c.postEvents(ctx, gzipBuffer, compression)
		}

		return c.postEvents(ctx, buf, compression)
	}

	return c.sentLogBatch(ctx, ld, send)
}

func encodeBodyEvents(zippers *sync.Pool, evs []*splunk.Event, disableCompression bool, minContentLengthCompression uint) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range evs {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
		buf.WriteString("\r\n\r\n")
	}
	return getReader(zippers, buf, disableCompression, minContentLengthCompression)
}

func encodeBody(zippers *sync.Pool, dps []*splunk.Event, disableCompression bool, minContentLengthCompression uint) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range dps {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
		buf.WriteString("\r\n\r\n")
	}
	return getReader(zippers, buf, disableCompression, minContentLengthCompression)
}

// avoid attempting to compress things that fit into a single ethernet frame
func getReader(zippers *sync.Pool, b *bytes.Buffer, disableCompression bool, minContentLengthCompression uint) (io.Reader, bool, error) {
	var err error
	if !disableCompression && b.Len() > int(minContentLengthCompression) {
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

func numMetricPoint(md pdata.Metrics) int {
	_, numPoints := md.MetricAndDataPointCount()
	return numPoints
}
