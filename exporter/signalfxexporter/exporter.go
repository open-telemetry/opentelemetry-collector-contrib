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
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

type signalfMetadataExporter struct {
	component.MetricsExporter
	pushMetadata func(metadata []*collection.MetadataUpdate) error
}

func (sme *signalfMetadataExporter) ConsumeMetadata(metadata []*collection.MetadataUpdate) error {
	return sme.pushMetadata(metadata)
}

type signalfxExporter struct {
	logger             *zap.Logger
	pushMetricsData    func(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error)
	pushMetadata       func(metadata []*collection.MetadataUpdate) error
	pushResourceLogs   func(ctx context.Context, ld pdata.ResourceLogs) (droppedLogRecords int, err error)
	hostMetadataSyncer *hostmetadata.Syncer
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
) (*signalfxExporter, error) {
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

	var hms *hostmetadata.Syncer
	if config.SyncHostMetadata {
		hms = hostmetadata.NewSyncer(logger, dimClient)
	}

	return &signalfxExporter{
		logger:             logger,
		pushMetricsData:    dpClient.pushMetricsData,
		pushMetadata:       dimClient.PushMetadata,
		hostMetadataSyncer: hms,
	}, nil
}

func newGzipPool() sync.Pool {
	return sync.Pool{New: func() interface{} {
		return gzip.NewWriter(nil)
	}}
}

func newEventExporter(config *Config, logger *zap.Logger) (*signalfxExporter, error) {
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

	return &signalfxExporter{
		logger:           logger,
		pushResourceLogs: eventClient.pushResourceLogs,
	}, nil
}

func (se *signalfxExporter) pushMetrics(ctx context.Context, md pdata.Metrics) (int, error) {
	numDroppedTimeSeries, err := se.pushMetricsData(ctx, md)
	if err == nil && se.hostMetadataSyncer != nil {
		se.hostMetadataSyncer.Sync(md)
	}
	return numDroppedTimeSeries, err
}

func (se *signalfxExporter) pushLogs(ctx context.Context, ld pdata.Logs) (int, error) {
	var numDroppedRecords int
	var err error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		dropped, thisErr := se.pushResourceLogs(ctx, rls.At(i))
		numDroppedRecords += dropped
		err = multierr.Append(err, thisErr)
	}

	return numDroppedRecords, err
}
