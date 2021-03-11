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

	body, compressed, err := encodeBody(&c.zippers, splunkDataPoints, c.config.DisableCompression)
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
	body, compressed, err := encodeBodyEvents(&c.zippers, splunkEvents, c.config.DisableCompression)
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

func subLog(ld pdata.Logs, logIdx logIndex) pdata.Logs {
	if logIdx.zero() {
		return ld
	}
	clone := ld.Clone().InternalRep()

	subset := *clone.Orig
	subset = subset[logIdx.origIdx:]
	subset[0].InstrumentationLibraryLogs = subset[0].InstrumentationLibraryLogs[logIdx.instIdx:]
	subset[0].InstrumentationLibraryLogs[0].Logs = subset[0].InstrumentationLibraryLogs[0].Logs[logIdx.logsIdx:]

	clone.Orig = &subset
	return pdata.LogsFromInternalRep(clone)
}

func (c *client) chunk(ctx context.Context, ld pdata.Logs, max int, send func(ctx context.Context, buf *bytes.Buffer) error) (numDroppedLogs int, err error) {
	// Provide 5000 overflow because it overruns the max content length then trims it block. Hopefully will prevent
	// extra allocation.
	buf := bytes.NewBuffer(make([]byte, 0, max+5_000))
	encoder := json.NewEncoder(buf)
	checkPoint := 0
	var logIdx logIndex

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				event := mapLogRecordToSplunkEvent(logs.At(k), c.config, c.logger)
				if err := encoder.Encode(event); err != nil {
					numDroppedLogs++
					continue
				}

				buf.WriteString("\r\n\r\n")

				if buf.Len() >= max {
					// May want to check if checkPoint == 0 ?
					buf.Truncate(checkPoint)
					if err := send(ctx, buf); err != nil {
						return numDroppedLogs, consumererror.PartialLogsError(err, subLog(ld, logIdx))
					}
					logIdx = logIndex{
						origIdx: i,
						instIdx: j,
						logsIdx: k,
					}
					buf.Reset()
					checkPoint = 0
				}

				checkPoint = buf.Len()
			}
		}
	}

	if buf.Len() > 0 {
		if err := send(ctx, buf); err != nil {
			return numDroppedLogs, consumererror.PartialLogsError(err, subLog(ld, logIdx))
		}
	}

	return numDroppedLogs, nil
}

func (c *client) pushLogData(ctx context.Context, ld pdata.Logs) (numDroppedLogs int, err error) {
	c.wg.Add(1)
	defer c.wg.Done()

	gzipWriter := c.zippers.Get().(*gzip.Writer)
	defer c.zippers.Put(gzipWriter)

	gzipBuf := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthLogs))
	gzipWriter.Reset(gzipBuf)
	defer gzipWriter.Close()

	// Callback when each chunk is to be sent.
	sendBatch := func(ctx context.Context, buf *bytes.Buffer) error {
		compression := false
		var reader io.Reader

		if buf.Len() < 1500 || c.config.DisableCompression {
			compression = false
			reader = buf
		} else {
			compression = true

			gzipBuf.Reset()
			gzipWriter.Reset(gzipBuf)

			if _, err := io.Copy(gzipWriter, buf); err != nil {
				return err
			}

			if err := gzipWriter.Flush(); err != nil {
				return err
			}

			reader = gzipBuf
		}

		if err = c.postEvents(ctx, reader, compression); err != nil {
			return err
		}

		return nil
	}

	return c.chunk(ctx, ld, c.config.MaxContentLengthLogs, sendBatch)
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
	if !disableCompression && b.Len() > 1500 {
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
