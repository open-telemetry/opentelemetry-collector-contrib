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
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// adxMetricsProducer uses the ADX client to perform ingestion
type adxMetricsProducer struct {
	client        *kusto.Client       // Shared client for logs , traces and metrics
	managedingest *ingest.Managed     // managed ingestion for metrics
	queuedingest  *ingest.Ingestion   // queued ingestion for metrics
	ingestoptions []ingest.FileOption // Options for the ingestion
	logger        *zap.Logger         // Loggers for tracing the flow
}

func (e *adxMetricsProducer) metricsDataPusher(_ context.Context, metrics pmetric.Metrics) error {
	resourceMetric := metrics.ResourceMetrics()
	for i := 0; i < resourceMetric.Len(); i++ {
		res := resourceMetric.At(i).Resource()
		scopeMetrics := resourceMetric.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				transformedadxmetrics := mapToAdxMetric(res, metrics.At(k), e.logger)
				for tm := 0; tm < len(transformedadxmetrics); tm++ {
					adxmetricjsonbytes, err := jsoniter.Marshal(transformedadxmetrics[tm])
					if err != nil {
						e.logger.Error("Error performing serialization of data.", zap.Error(err))
					}
					ingestreader := bytes.NewReader(adxmetricjsonbytes)
					if e.managedingest != nil {
						if _, err := e.managedingest.FromReader(context.Background(), ingestreader, e.ingestoptions...); err != nil {
							e.logger.Error("Error performing managed data ingestion.", zap.Error(err))
							return err
						}
					} else {
						if _, err := e.queuedingest.FromReader(context.Background(), ingestreader, e.ingestoptions...); err != nil {
							e.logger.Error("Error performing queued data ingestion.", zap.Error(err))
							return err
						}
					}

				}
			}
		}
	}
	return nil
}

func (amp *adxMetricsProducer) Close(context.Context) error {
	return amp.managedingest.Close()
}

/*
Create a metric exporter. The metric exporter instantiates a client , creates the ingester and then sends data through it
*/
func newMetricsExporter(config *Config, logger *zap.Logger) (*adxMetricsProducer, error) {
	metricclient, err := buildAdxClient(config)

	if err != nil {
		return nil, err
	}

	var managedingest *ingest.Managed
	var queuedingest *ingest.Ingestion

	// The exporter could be configured to run in either modes. Using managedstreaming or batched queueing
	if strings.ToLower(config.IngestionType) == managedingesttype {
		mi, err := createManagedStreamingIngester(config, metricclient, config.RawMetricTable)
		if err != nil {
			return nil, err
		}

		managedingest = mi
		queuedingest = nil
		err = nil
	} else {
		qi, err := createQueuedIngester(config, metricclient, config.RawMetricTable)
		if err != nil {
			return nil, err
		}
		managedingest = nil
		queuedingest = qi
		err = nil
	}
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	// Expect that this mapping is alreay existent
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.JSON)
	return &adxMetricsProducer{
		client:        metricclient,
		managedingest: managedingest,
		queuedingest:  queuedingest,
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
func createManagedStreamingIngester(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Managed, error) {
	//ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.MultiJSON)
	ingester, err := ingest.NewManaged(adxclient, config.Database, tablename)
	return ingester, err
}

// A queued ingester in case that is provided as the config option
func createQueuedIngester(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Ingestion, error) {
	//ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.MultiJSON)
	ingester, err := ingest.New(adxclient, config.Database, tablename)
	return ingester, err
}
