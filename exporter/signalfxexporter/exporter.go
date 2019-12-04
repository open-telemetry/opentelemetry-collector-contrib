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

package signalfxexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumererror"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"go.uber.org/zap"
)

// New returns a new SignalFx exporter.
func New(
	config *Config,
	logger *zap.Logger,
) (exporter.MetricsExporter, error) {

	if config == nil {
		return nil, errors.New("nil config")
	}

	if config.Name() == "" {
		config.SetType(typeStr)
		config.SetName(typeStr)
	}

	if config.Realm == "" && config.URL == "" {
		err := fmt.Errorf(
			"%q config requires a non-empty \"realm\" or \"url\"",
			config.Name())
		return nil, err
	}

	var actualURL string
	if config.URL == "" {
		actualURL = fmt.Sprintf(
			"https://ingest.%s.signalfx.com/v2/datapoint", config.Realm)
	} else {
		// Ignore realm and use the URL. Typically used for debugging.
		u, err := url.Parse(config.URL)
		if err != nil {
			return nil, fmt.Errorf(
				"%q invalid \"url\": %v", config.Name(), err)
		}
		if u.Path == "" || u.Path == "/" {
			u.Path = path.Join(u.Path, "v2/datapoint")
		}
		actualURL = u.String()
		logger.Info("SingalFX Config", zap.String("actual_url", actualURL))
	}

	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	if config.Timeout < 0 {
		err := fmt.Errorf(
			"%q config cannot have a negative \"timeout\"",
			config.Name())
		return nil, err
	}

	headers, err := buildHeaders(config)
	if err != nil {
		return nil, err
	}

	s := &httpSender{
		url:     actualURL,
		headers: headers,
		client: &http.Client{
			// TODO: What other settings of http.Client to expose via config?
			//  Or what others change from default values?
			Timeout: config.Timeout,
		},
		logger: logger,
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
	}

	exp, err := exporterhelper.NewMetricsExporter(
		&config.ExporterSettings,
		s.pushMetricsData,
		exporterhelper.WithTracing(true),
		exporterhelper.WithMetrics(true))

	return exp, err
}

// httpSender sends the data to the SignalFx backend.
type httpSender struct {
	url     string
	headers map[string]string
	client  *http.Client
	logger  *zap.Logger
	zippers sync.Pool
}

func (s *httpSender) pushMetricsData(
	ctx context.Context,
	md consumerdata.MetricsData,
) (droppedTimeSeries int, err error) {

	sfxDataPoints, numDroppedTimeseries, err := metricDataToSingalFxV2(s.logger, md)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	body, compressed, err := s.encodeBody(sfxDataPoints)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	req, err := http.NewRequest("POST", s.url, body)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// SignalFx accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return exporterhelper.NumTimeSeries(md), err
	}

	return numDroppedTimeseries, nil
}

func buildHeaders(config *Config) (map[string]string, error) {
	headers := map[string]string{
		"Connection":   "keep-alive",
		"Content-Type": "application/x-protobuf",
		"User-Agent":   "OpenTelemetry-Collector SignalFx Exporter/v0.0.1",
	}

	if config.AccessToken != "" {
		headers["X-Sf-Token"] = config.AccessToken
	}

	// Add any custom headers from the config. They will override the pre-defined
	// ones above in case of conflict, but, not the content encoding one since
	// the latter one is defined according to the payload.
	for k, v := range config.Headers {
		headers[k] = v
	}

	return headers, nil
}

func (s *httpSender) encodeBody(dps []*sfxpb.DataPoint) (bodyReader io.Reader, compressed bool, err error) {
	msg := &sfxpb.DataPointUploadMessage{
		Datapoints: dps,
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}

// avoid attempting to compress things that fit into a single ethernet frame
func (s *httpSender) getReader(b []byte) (io.Reader, bool, error) {
	var err error
	if len(b) > 1500 {
		buf := new(bytes.Buffer)
		w := s.zippers.Get().(*gzip.Writer)
		defer s.zippers.Put(w)
		w.Reset(buf)
		_, err = w.Write(b)
		if err == nil {
			err = w.Close()
			if err == nil {
				return buf, true, nil
			}
		}
	}
	return bytes.NewReader(b), false, err
}
