// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// adxMetricsProducer uses the ADX client to perform ingestion
type adxMetricsProducer struct {
	config        *Config
	client        *kusto.Client
	ingest        *ingest.Ingestion
	ingestoptions []ingest.FileOption
	logger        *zap.Logger
}

func (e *adxMetricsProducer) metricsDataPusher(_ context.Context, metrics pmetric.Metrics) error {
	resourceMetric := metrics.ResourceMetrics()
	for i := 0; i < resourceMetric.Len(); i++ {
		res := resourceMetric.At(i).Resource()
		scopeMetrics := resourceMetric.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				mapToAdxMetric(res, metrics.At(k), e.config, e.logger)
			}
		}
	}
	buf := new(bytes.Buffer)
	r, w := io.Pipe()
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				fmt.Printf("Failed to close writer %v", err)
			}
		}(w)
		_, err := io.Copy(w, buf)
		if err != nil {
			fmt.Printf("Failed to copy io: %v", err)
		}
	}()

	if _, err := e.ingest.FromReader(context.Background(), r, e.ingestoptions...); err != nil {
		logger.Error("Error performing data ingestion.", zap.Error(err))
		return err
	}
	return nil
}

func (amp *adxMetricsProducer) Close(context.Context) error {
	return amp.ingest.Close()
}

/*
Create a metric exporter. The metric exporter instantiates a client , creates the ingester and then sends data through it
*/
func newMetricsExporter(config *Config, logger *zap.Logger) (*adxMetricsProducer, error) {
	metricclient, err := buildAdxClient(config)

	if err != nil {
		return nil, err
	}

	metricingest, err := createIngester(config, metricclient, config.RawMetricTable)

	if err != nil {
		return nil, err
	}
	//TODO much more optimized as a multi line JSON
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.MultiJSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.MultiJSON)

	return &adxMetricsProducer{
		config:        config,
		client:        metricclient,
		ingest:        metricingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}, nil

}

/**
Common functions that are used by all the 3 parts of OTEL , namely Traces , Logs and Metrics

*/

func buildAdxClient(config *Config) (*kusto.Client, error) {
	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientId,
			config.ClientSecret, config.TenantId),
	}
	client, err := kusto.New(config.ClusterName, authorizer)
	return client, err
}

// Depending on the table , create seperate ingesters
func createIngester(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Ingestion, error) {
	ingester, err := ingest.New(adxclient, config.Database, tablename)
	return ingester, err
}
