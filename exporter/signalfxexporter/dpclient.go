// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	contentEncodingHeader   = "Content-Encoding"
	contentTypeHeader       = "Content-Type"
	otlpProtobufContentType = "application/x-protobuf;format=otlp"
)

type sfxClientBase struct {
	ingestURL *url.URL
	headers   map[string]string
	client    *http.Client
	zippers   sync.Pool
}

var metricsMarshaler = &pmetric.JSONMarshaler{}

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
	logDataPoints          bool
	logger                 *zap.Logger
	accessTokenPassthrough bool
	converter              *translation.MetricsConverter
	sendOTLPHistograms     bool
}

func (s *sfxDPClient) pushMetricsData(
	ctx context.Context,
	md pmetric.Metrics,
) (droppedDataPoints int, err error) {
	rms := md.ResourceMetrics()
	if rms.Len() == 0 {
		return 0, nil
	}

	if s.logDataPoints {
		buf, err := metricsMarshaler.MarshalMetrics(md)
		if err != nil {
			s.logger.Error("Failed to marshal metrics for logging", zap.Error(err))
		} else {
			s.logger.Debug("received metrics", zap.String("pdata", string(buf)))
		}
	}

	// All metrics in the pmetric.Metrics will have the same access token because of the BatchPerResourceMetrics.
	metricToken := s.retrieveAccessToken(ctx, rms.At(0))

	// export SFx format
	sfxDataPoints := s.converter.MetricsToSignalFxV2(md)
	if len(sfxDataPoints) > 0 {
		droppedCount, err := s.pushMetricsDataForToken(ctx, sfxDataPoints, metricToken)
		if err != nil {
			return droppedCount, err
		}
	}

	// export any histograms in otlp if sendOTLPHistograms is true
	if s.sendOTLPHistograms {
		histogramData, metricCount := getHistograms(md)
		if metricCount > 0 {
			droppedCount, err := s.pushOTLPMetricsDataForToken(ctx, histogramData, metricToken)
			if err != nil {
				return droppedCount, err
			}
		}
	}

	return 0, nil
}

func (s *sfxDPClient) postData(ctx context.Context, body io.Reader, headers map[string]string) error {
	datapointURL := *s.ingestURL
	if !strings.HasSuffix(datapointURL.Path, "v2/datapoint") {
		datapointURL.Path = path.Join(datapointURL.Path, "v2/datapoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, datapointURL.String(), body)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// Set the headers configured in sfxDPClient
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	// Set any extra headers passed by the caller
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// TODO: Mark errors as partial errors wherever applicable when, partial
	// error for metrics is available.
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}
	return nil
}

func (s *sfxDPClient) pushMetricsDataForToken(ctx context.Context, sfxDataPoints []*sfxpb.DataPoint, accessToken string) (int, error) {
	if s.logDataPoints {
		for _, dp := range sfxDataPoints {
			s.logger.Debug("Dispatching SFx datapoint", zap.Stringer("dp", dp))
		}
	}

	body, compressed, err := s.encodeBody(sfxDataPoints)
	dataPointCount := len(sfxDataPoints)
	if err != nil {
		return dataPointCount, consumererror.NewPermanent(err)
	}

	headers := make(map[string]string)

	// Override access token in headers map if it's non empty.
	if accessToken != "" {
		headers[splunk.SFxAccessTokenHeader] = accessToken
	}

	if compressed {
		headers[contentEncodingHeader] = "gzip"
	}

	err = s.postData(ctx, body, headers)
	if err != nil {
		return dataPointCount, err
	}
	return 0, nil
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

func (s *sfxDPClient) retrieveAccessToken(ctx context.Context, md pmetric.ResourceMetrics) string {
	if !s.accessTokenPassthrough {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	cl := client.FromContext(ctx)
	ss := cl.Metadata.Get(splunk.SFxAccessTokenHeader)
	if len(ss) > 0 {
		return ss[0]
	}

	attrs := md.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok {
		return accessToken.Str()
	}
	return ""
}

func (s *sfxDPClient) pushOTLPMetricsDataForToken(ctx context.Context, mh pmetric.Metrics, accessToken string) (int, error) {
	dataPointCount := mh.DataPointCount()
	if s.logDataPoints {
		s.logger.Debug("Count of metrics to send in OTLP format",
			zap.Int("resource metrics", mh.ResourceMetrics().Len()),
			zap.Int("metrics", mh.MetricCount()),
			zap.Int("data points", dataPointCount))
		buf, err := metricsMarshaler.MarshalMetrics(mh)
		if err != nil {
			s.logger.Error("Failed to marshal metrics for logging otlp histograms", zap.Error(err))
		} else {
			s.logger.Debug("Dispatching OTLP metrics", zap.String("pmetrics", string(buf)))
		}
	}

	body, compressed, err := s.encodeOTLPBody(mh)
	if err != nil {
		return dataPointCount, consumererror.NewPermanent(err)
	}

	headers := make(map[string]string)

	// Set otlp content-type header
	headers[contentTypeHeader] = otlpProtobufContentType

	// Override access token in headers map if it's non-empty.
	if accessToken != "" {
		headers[splunk.SFxAccessTokenHeader] = accessToken
	}

	if compressed {
		headers[contentEncodingHeader] = "gzip"
	}

	s.logger.Debug("Sending metrics in OTLP format")

	err = s.postData(ctx, body, headers)
	if err != nil {
		return dataPointCount, consumererror.NewMetrics(err, mh)
	}

	return 0, nil
}

func (s *sfxDPClient) encodeOTLPBody(md pmetric.Metrics) (bodyReader io.Reader, compressed bool, err error) {
	tr := pmetricotlp.NewExportRequestFromMetrics(md)

	body, err := tr.MarshalProto()
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}
