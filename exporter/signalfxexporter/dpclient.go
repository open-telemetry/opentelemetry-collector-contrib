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

package signalfxexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type sfxClientBase struct {
	ingestURL *url.URL
	headers   map[string]string
	client    *http.Client
	zippers   sync.Pool
}

// avoid attempting to compress things that fit into a single ethernet frame
func (s *sfxClientBase) getReader(b []byte) (io.Reader, bool, error) {
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

// sfxDPClient sends the data to the SignalFx backend.
type sfxDPClient struct {
	sfxClientBase
	logger                 *zap.Logger
	accessTokenPassthrough bool
	converter              *translation.MetricsConverter
}

func (s *sfxDPClient) pushMetricsData(
	ctx context.Context,
	md pdata.Metrics,
) (droppedDataPoints int, err error) {
	rms := md.ResourceMetrics()
	if rms.Len() == 0 {
		return 0, nil
	}

	// All metrics in the pdata.Metrics will have the same access token because of the BatchPerResourceMetrics.
	metricToken := s.retrieveAccessToken(rms.At(0))

	var sfxDataPoints []*sfxpb.DataPoint

	for i := 0; i < rms.Len(); i++ {
		sfxDataPoints = append(sfxDataPoints, s.converter.MetricDataToSignalFxV2(rms.At(i))...)
	}

	return s.pushMetricsDataForToken(ctx, sfxDataPoints, metricToken)
}

func (s *sfxDPClient) pushMetricsDataForToken(ctx context.Context, sfxDataPoints []*sfxpb.DataPoint, accessToken string) (int, error) {
	body, compressed, err := s.encodeBody(sfxDataPoints)
	if err != nil {
		return len(sfxDataPoints), consumererror.NewPermanent(err)
	}

	datapointURL := *s.ingestURL
	if !strings.HasSuffix(datapointURL.Path, "v2/datapoint") {
		datapointURL.Path = path.Join(datapointURL.Path, "v2/datapoint")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", datapointURL.String(), body)
	if err != nil {
		return len(sfxDataPoints), consumererror.NewPermanent(err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	// Override access token in headers map if it's non empty.
	if accessToken != "" {
		req.Header.Set(splunk.SFxAccessTokenHeader, accessToken)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// TODO: Mark errors as partial errors wherever applicable when, partial
	// error for metrics is available.
	resp, err := s.client.Do(req)
	if err != nil {
		return len(sfxDataPoints), err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return len(sfxDataPoints), err
	}
	return 0, nil
}

func buildHeaders(config *Config) map[string]string {
	headers := map[string]string{
		"Connection":   "keep-alive",
		"Content-Type": "application/x-protobuf",
		"User-Agent":   "OpenTelemetry-Collector SignalFx Exporter/v0.0.1",
	}

	if config.AccessToken != "" {
		headers[splunk.SFxAccessTokenHeader] = config.AccessToken
	}

	// Add any custom headers from the config. They will override the pre-defined
	// ones above in case of conflict, but, not the content encoding one since
	// the latter one is defined according to the payload.
	for k, v := range config.Headers {
		headers[k] = v
	}

	return headers
}

func (s *sfxDPClient) encodeBody(dps []*sfxpb.DataPoint) (bodyReader io.Reader, compressed bool, err error) {
	msg := sfxpb.DataPointUploadMessage{
		Datapoints: dps,
	}
	body, err := msg.Marshal()
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}

func (s *sfxDPClient) retrieveAccessToken(md pdata.ResourceMetrics) string {
	if !s.accessTokenPassthrough {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	attrs := md.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok {
		return accessToken.StringVal()
	}
	return ""
}
