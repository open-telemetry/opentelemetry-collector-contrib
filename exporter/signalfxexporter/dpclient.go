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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
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
) (droppedTimeSeries int, err error) {
	metricsData := internaldata.MetricsToOC(md)
	var numDroppedTimeseries int
	var errs []error

	var currentToken string
	var batchStartIdx int
	for i := range metricsData {
		metricToken := s.retrieveAccessToken(metricsData[i])
		if currentToken != metricToken {
			if batchStartIdx < i {
				// TODO:
				// 1) To leverage the queued retry mechanism available in new exporters,
				// 	  look for error cases that are non-permanent and track the list of metric
				//    data that needs to be retried. This will be done once partial retry support
				//    is added for metrics type.
				// 2) Also consider invoking in a goroutine to some concurrency going, in case
				//    there are a lot of tokens.
				droppedCount, err := s.pushMetricsDataForToken(ctx, metricsData[batchStartIdx:i], currentToken)
				numDroppedTimeseries += droppedCount
				if err != nil {
					errs = append(errs, err)
				}
				batchStartIdx = i
			}
			currentToken = metricToken
		}
	}

	// Ensure to get the last chunk of metrics.
	if len(metricsData[batchStartIdx:]) > 0 {
		droppedCount, err := s.pushMetricsDataForToken(ctx, metricsData[batchStartIdx:], currentToken)
		numDroppedTimeseries += droppedCount
		if err != nil {
			errs = append(errs, err)
		}
	}

	return numDroppedTimeseries, componenterror.CombineErrors(errs)
}

func (s *sfxDPClient) pushMetricsDataForToken(ctx context.Context,
	metricsData []consumerdata.MetricsData, accessToken string) (int, error) {
	numTimeseries := timeseriesCount(metricsData)
	sfxDataPoints := make([]*sfxpb.DataPoint, 0, numTimeseries)
	sfxDataPoints, numDroppedTimeseries := s.converter.MetricDataToSignalFxV2(metricsData, sfxDataPoints)

	body, compressed, err := s.encodeBody(sfxDataPoints)
	if err != nil {
		return numTimeseries, consumererror.Permanent(err)
	}

	datapointURL := *s.ingestURL
	if !strings.HasSuffix(datapointURL.Path, "v2/datapoint") {
		datapointURL.Path = path.Join(datapointURL.Path, "v2/datapoint")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", datapointURL.String(), body)
	if err != nil {
		return numTimeseries, consumererror.Permanent(err)
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
		return numTimeseries, err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// SignalFx accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return numTimeseries, err
	}

	return numDroppedTimeseries, nil

}

func timeseriesCount(metricsData []consumerdata.MetricsData) int {
	numTimeseries := 0
	for _, metricData := range metricsData {
		numTimeseries += exporterhelper.NumTimeSeries(metricData)
	}
	return numTimeseries
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

func (s *sfxDPClient) retrieveAccessToken(md consumerdata.MetricsData) string {
	if !s.accessTokenPassthrough || md.Resource == nil {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	accessToken := ""
	if labels := md.Resource.GetLabels(); labels != nil {
		accessToken = labels[splunk.SFxAccessTokenLabel]
		// Drop internally passed access token in all cases
		delete(labels, splunk.SFxAccessTokenLabel)
	}
	return accessToken
}
