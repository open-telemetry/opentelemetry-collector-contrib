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
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

const (
	idleConnTimeout     = 30 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
	dialerTimeout       = 30 * time.Second
	dialerKeepAlive     = 30 * time.Second
)

// TraceAndMetricExporter sends traces and metrics.
type TraceAndMetricExporter interface {
	component.Component
	consumer.MetricsConsumerOld
	consumer.TraceConsumerOld
}

type splunkExporter struct {
	pushMetricsData func(ctx context.Context, md consumerdata.MetricsData) (droppedTimeSeries int, err error)
	pushTraceData   func(ctx context.Context, td consumerdata.TraceData) (numDroppedSpans int, err error)
	stop            func(ctx context.Context) (err error)
}

type exporterOptions struct {
	url   *url.URL
	token string
}

// createExporter returns a new Splunk exporter.
func createExporter(
	config *Config,
	logger *zap.Logger,
) (TraceAndMetricExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil,
			fmt.Errorf("failed to process %q config: %v", config.Name(), err)
	}

	client := buildClient(options, config, logger)

	return splunkExporter{
		pushMetricsData: client.pushMetricsData,
		pushTraceData:   client.pushTraceData,
		stop:            client.stop,
	}, nil
}

func buildClient(options *exporterOptions, config *Config, logger *zap.Logger) *client {
	return &client{
		url: options.url,
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   dialerTimeout,
					KeepAlive: dialerKeepAlive,
				}).DialContext,
				MaxIdleConns:        int(config.MaxConnections),
				MaxIdleConnsPerHost: int(config.MaxConnections),
				IdleConnTimeout:     idleConnTimeout,
				TLSHandshakeTimeout: tlsHandshakeTimeout,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: config.InsecureSkipVerify,
				},
			},
		},
		logger: logger,
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		headers: map[string]string{
			"Connection":    "keep-alive",
			"Content-Type":  "application/json",
			"User-Agent":    "OpenTelemetry-Collector Splunk Exporter/v0.0.1",
			"Authorization": "Splunk " + config.Token,
		},
		config: config,
	}
}

func (se splunkExporter) Start(context.Context, component.Host) error {
	return nil
}

func (se splunkExporter) Shutdown(ctxt context.Context) error {
	return se.stop(ctxt)
}

func (se splunkExporter) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	ctx = obsreport.StartMetricsExportOp(ctx, typeStr)
	numDroppedTimeSeries, err := se.pushMetricsData(ctx, md)

	numReceivedTimeSeries, numPoints := pdatautil.TimeseriesAndPointCount(md)

	obsreport.EndMetricsExportOp(ctx, numPoints, numReceivedTimeSeries, numDroppedTimeSeries, err)
	return err
}

func (se splunkExporter) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	ctx = obsreport.StartTraceDataExportOp(ctx, typeStr)

	numDroppedSpans, err := se.pushTraceData(ctx, td)

	obsreport.EndTraceDataExportOp(ctx, len(td.Spans), numDroppedSpans, err)
	return err
}
