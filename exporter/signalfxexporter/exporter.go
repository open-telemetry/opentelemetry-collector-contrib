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

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// TODO: Find a place for this to be shared.
type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

// TODO: Find a place for this to be shared.
type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

type signalfMetadataExporter struct {
	component.MetricsExporter
	pushMetadata func(metadata []*metadata.MetadataUpdate) error
}

func (sme *signalfMetadataExporter) ConsumeMetadata(metadata []*metadata.MetadataUpdate) error {
	return sme.pushMetadata(metadata)
}

type metricExporter struct {
	pushMetricsData    func(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error)
	pushMetadata       func(metadata []*metadata.MetadataUpdate) error
	hostMetadataSyncer *hostmetadata.Syncer
	config             *Config
	logger             *zap.Logger
}

type logExporter struct {
	pushLogsData func(ctx context.Context, ld pdata.Logs) (droppedLogRecords int, err error)
	config       *Config
	logger       *zap.Logger
}

type exporterOptions struct {
	ingestURL        *url.URL
	apiURL           *url.URL
	httpTimeout      time.Duration
	token            string
	logDataPoints    bool
	logDimUpdate     bool
	metricTranslator *translation.MetricTranslator
}

func newMetricExporter(config *Config, logger *zap.Logger) *metricExporter {
	return &metricExporter{
		config: config,
		logger: logger,
	}
}

func (me *metricExporter) Start(_ context.Context, host component.Host) (err error) {
	return me.newSignalFxExporter(host)
}

// newSignalFxExporter initializes a new SignalFx exporter.
func (me *metricExporter) newSignalFxExporter(host component.Host) error {
	if me.config == nil {
		return errors.New("nil config")
	}

	options, err := me.config.getOptionsFromConfig()
	if err != nil {
		return fmt.Errorf("failed to process %q config: %v", me.config.ID().String(), err)
	}

	headers := buildHeaders(me.config)
	converter, err := translation.NewMetricsConverter(me.logger, options.metricTranslator, me.config.ExcludeMetrics, me.config.IncludeMetrics, me.config.NonAlphanumericDimensionChars)
	if err != nil {
		return fmt.Errorf("failed to create metric converter: %v", err)
	}

	defaultIdleConnTimeout := 30 * time.Second
	httpClient := confighttp.HTTPClientSettings{
		MaxIdleConns:        &me.config.MaxConnections,
		MaxIdleConnsPerHost: &me.config.MaxConnections,
		IdleConnTimeout:     &defaultIdleConnTimeout,
		Timeout:             me.config.Timeout,
	}

	client, err := httpClient.ToClient(host.GetExtensions())
	if err != nil {
		return fmt.Errorf("failed to create http client: %v", err)
	}

	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: options.ingestURL,
			headers:   headers,
			client:    client,
			zippers:   newGzipPool(),
		},
		logDataPoints:          options.logDataPoints,
		logger:                 me.logger,
		accessTokenPassthrough: me.config.AccessTokenPassthrough,
		converter:              converter,
	}

	dimClient := dimensions.NewDimensionClient(
		context.Background(),
		dimensions.DimensionClientOptions{
			Token:      options.token,
			APIURL:     options.apiURL,
			LogUpdates: options.logDimUpdate,
			Logger:     me.logger,
			// Duration to wait between property updates. This might be worth
			// being made configurable.
			SendDelay: 10,
			// In case of having issues sending dimension updates to SignalFx,
			// buffer a fixed number of updates. Might also be a good candidate
			// to make configurable.
			PropertiesMaxBuffered: 10000,
			MetricsConverter:      *converter,
		})
	dimClient.Start()

	var hms *hostmetadata.Syncer
	if me.config.SyncHostMetadata {
		hms = hostmetadata.NewSyncer(me.logger, dimClient)
	}

	me.pushMetricsData = dpClient.pushMetricsData
	me.pushMetadata = dimClient.PushMetadata
	me.hostMetadataSyncer = hms

	return nil
}

func newGzipPool() sync.Pool {
	return sync.Pool{New: func() interface{} {
		return gzip.NewWriter(nil)
	}}
}

func newLogExporter(config *Config, logger *zap.Logger) *logExporter {
	return &logExporter{
		config: config,
		logger: logger,
	}
}

func (le *logExporter) Start(_ context.Context, host component.Host) (err error) {
	return le.newEventExporter(host)
}

func (le *logExporter) newEventExporter(host component.Host) error {
	if le.config == nil {
		return errors.New("nil config")
	}

	options, err := le.config.getOptionsFromConfig()
	if err != nil {
		return fmt.Errorf("failed to process %q config: %v", le.config.ID().String(), err)
	}

	headers := buildHeaders(le.config)

	defaultIdleConnTimeout := 30 * time.Second
	httpClient := confighttp.HTTPClientSettings{
		MaxIdleConns:        &le.config.MaxConnections,
		MaxIdleConnsPerHost: &le.config.MaxConnections,
		IdleConnTimeout:     &defaultIdleConnTimeout,
		Timeout:             le.config.Timeout,
	}

	client, err := httpClient.ToClient(host.GetExtensions())
	if err != nil {
		return fmt.Errorf("failed to create http client: %v", err)
	}

	eventClient := &sfxEventClient{
		sfxClientBase: sfxClientBase{
			ingestURL: options.ingestURL,
			headers:   headers,
			client:    client,
			zippers:   newGzipPool(),
		},
		logger:                 le.logger,
		accessTokenPassthrough: le.config.AccessTokenPassthrough,
	}

	le.pushLogsData = eventClient.pushLogsData
	return nil
}

func (me *metricExporter) pushMetrics(ctx context.Context, md pdata.Metrics) error {
	_, err := me.pushMetricsData(ctx, md)
	if err == nil && me.hostMetadataSyncer != nil {
		me.hostMetadataSyncer.Sync(md)
	}
	return err
}

func (le *logExporter) pushLogs(ctx context.Context, ld pdata.Logs) error {
	_, err := le.pushLogsData(ctx, ld)
	return err
}
