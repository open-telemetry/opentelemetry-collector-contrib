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
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

type signalfxExporter struct {
	logger                 *zap.Logger
	pushMetricsData        func(ctx context.Context, md consumerdata.MetricsData) (droppedTimeSeries int, err error)
	pushKubernetesMetadata func(metadata []*collection.KubernetesMetadataUpdate) error
}

type exporterOptions struct {
	ingestURL    *url.URL
	apiURL       *url.URL
	httpTimeout  time.Duration
	token        string
	logDimUpdate bool
}

// New returns a new SignalFx exporter.
func New(
	config *Config,
	logger *zap.Logger,
) (component.MetricsExporterOld, error) {

	if config == nil {
		return nil, errors.New("nil config")
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil,
			fmt.Errorf("failed to process %q config: %v", config.Name(), err)
	}

	headers, err := buildHeaders(config)
	if err != nil {
		return nil, err
	}

	dpClient := &sfxDPClient{
		ingestURL: options.ingestURL,
		headers:   headers,
		client: &http.Client{
			// TODO: What other settings of http.Client to expose via config?
			//  Or what others change from default values?
			Timeout: config.Timeout,
		},
		logger: logger,
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		accessTokenPassthrough: config.AccessTokenPassthrough,
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
		})
	dimClient.Start()

	return signalfxExporter{
		logger:                 logger,
		pushMetricsData:        dpClient.pushMetricsData,
		pushKubernetesMetadata: dimClient.PushKubernetesMetadata,
	}, nil
}

func (se signalfxExporter) Start(context.Context, component.Host) error {
	return nil
}

func (se signalfxExporter) Shutdown(context.Context) error {
	return nil
}

func (se signalfxExporter) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	ctx = obsreport.StartMetricsExportOp(ctx, typeStr)
	numDroppedTimeSeries, err := se.pushMetricsData(ctx, md)

	numReceivedTimeSeries, numPoints := pdatautil.TimeseriesAndPointCount(md)

	obsreport.EndMetricsExportOp(ctx, numPoints, numReceivedTimeSeries, numDroppedTimeSeries, err)
	return err
}

func (se signalfxExporter) ConsumeKubernetesMetadata(metadata []*collection.KubernetesMetadataUpdate) error {
	return se.pushKubernetesMetadata(metadata)
}
