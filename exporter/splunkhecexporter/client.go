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

	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

// client sends the data to the splunk backend.
type client struct {
	config  *Config
	url     *url.URL
	client  *http.Client
	logger  *zap.Logger
	zippers sync.Pool
}

func (c *client) pushMetricsData(
	ctx context.Context,
	md consumerdata.MetricsData,
) (droppedTimeSeries int, err error) {

	splunkDataPoints, numDroppedTimeseries, err := metricDataToSplunk(c.logger, md, c.config)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	body, compressed, err := c.encodeBody(splunkDataPoints)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	req, err := http.NewRequest("POST", c.url.String(), body)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}
	headers := buildHeaders(c.config)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// Splunk accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return exporterhelper.NumTimeSeries(md), err
	}

	return numDroppedTimeseries, nil
}

func buildHeaders(config *Config) map[string]string {
	headers := map[string]string{
		"Connection":   "keep-alive",
		"Content-Type": "application/json",
		"User-Agent":   "OpenTelemetry-Collector Splunk Exporter/v0.0.1",
	}

	if config.Token != "" {
		headers["Authorization"] = "Splunk " + config.Token
	}

	return headers
}

func (c *client) encodeBody(dps []*splunkMetric) (bodyReader io.Reader, compressed bool, err error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for _, e := range dps {
		err := encoder.Encode(e)
		if err != nil {
			return nil, false, err
		}
		buf.WriteString("\r\n\r\n")
	}
	return c.getReader(buf)
}

// avoid attempting to compress things that fit into a single ethernet frame
func (c *client) getReader(b *bytes.Buffer) (io.Reader, bool, error) {
	var err error
	if b.Len() > 1500 {
		buf := new(bytes.Buffer)
		w := c.zippers.Get().(*gzip.Writer)
		defer c.zippers.Put(w)
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
