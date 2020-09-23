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
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

type signalfxExporter struct {
	logger           *zap.Logger
	pushMetricsData  func(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error)
	pushMetadata     func(metadata []*collection.MetadataUpdate) error
	pushResourceLogs func(ctx context.Context, ld pdata.ResourceLogs) (droppedLogRecords int, err error)
}

type exporterOptions struct {
	ingestURL        *url.URL
	apiURL           *url.URL
	httpTimeout      time.Duration
	token            string
	logDimUpdate     bool
	metricTranslator *translation.MetricTranslator
}

// newSignalFxExporter returns a new SignalFx exporter.
func newSignalFxExporter(
	config *Config,
	logger *zap.Logger,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil,
			fmt.Errorf("failed to process %q config: %v", config.Name(), err)
	}

	headers := buildHeaders(config)

	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: options.ingestURL,
			headers:   headers,
			client: &http.Client{
				// TODO: What other settings of http.Client to expose via config?
				//  Or what others change from default values?
				Timeout: config.Timeout,
			},
			zippers: newGzipPool(),
		},
		logger:                 logger,
		accessTokenPassthrough: config.AccessTokenPassthrough,
		converter:              translation.NewMetricsConverter(logger, options.metricTranslator),
	}

	dimClient := dimensions.NewDimensionClient(
		context.Background(),
		dimensions.DimensionClientOptions{
			Token:      options.token,
			APIURL:     options.apiURL,
			LogUpdates: options.logDimUpdate,
			Logger:     logger,
			// Duration to wait between property updates. This might be worth
			// being made configurable.
			SendDelay: 10,
			// In case of having issues sending dimension updates to SignalFx,
			// buffer a fixed number of updates. Might also be a good candidate
			// to make configurable.
			PropertiesMaxBuffered: 10000,
			MetricTranslator:      options.metricTranslator,
		})
	dimClient.Start()

	return signalfxExporter{
		logger:          logger,
		pushMetricsData: dpClient.pushMetricsData,
		pushMetadata:    dimClient.PushMetadata,
	}, nil
}

func newGzipPool() sync.Pool {
	return sync.Pool{New: func() interface{} {
		return gzip.NewWriter(nil)
	}}
}

func NewEventExporter(config *Config, logger *zap.Logger) (component.LogsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil,
			fmt.Errorf("failed to process %q config: %v", config.Name(), err)
	}

	headers := buildHeaders(config)

	eventClient := &sfxEventClient{
		sfxClientBase: sfxClientBase{
			ingestURL: options.ingestURL,
			headers:   headers,
			client: &http.Client{
				// TODO: What other settings of http.Client to expose via config?
				//  Or what others change from default values?
				Timeout: config.Timeout,
			},
			zippers: newGzipPool(),
		},
		logger:                 logger,
		accessTokenPassthrough: config.AccessTokenPassthrough,
	}

	return signalfxExporter{
		logger:           logger,
		pushResourceLogs: eventClient.pushResourceLogs,
	}, nil
}

func (se signalfxExporter) Start(context.Context, component.Host) error {
	return nil
}

func (se signalfxExporter) Shutdown(context.Context) error {
	return nil
}

func (se signalfxExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	ctx = obsreport.StartMetricsExportOp(ctx, typeStr)
	numDroppedTimeSeries, err := se.pushMetricsData(ctx, md)
	numReceivedTimeSeries, numPoints := md.MetricAndDataPointCount()

	obsreport.EndMetricsExportOp(ctx, numPoints, numReceivedTimeSeries, numDroppedTimeSeries, err)
	return err
}

func (se signalfxExporter) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	ctx = obsreport.StartLogsExportOp(ctx, typeStr)

	var numDroppedRecords int
	var err error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		dropped, thisErr := se.pushResourceLogs(ctx, rls.At(i))
		numDroppedRecords += dropped
		err = multierr.Append(err, thisErr)
	}

	obsreport.EndLogsExportOp(ctx, ld.LogRecordCount(), numDroppedRecords, err)
	return err
}

func (se signalfxExporter) ConsumeMetadata(metadata []*collection.MetadataUpdate) error {
	return se.pushMetadata(metadata)
}
