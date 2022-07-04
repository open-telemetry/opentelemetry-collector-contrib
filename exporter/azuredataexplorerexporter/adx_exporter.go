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
	"context"
	"errors"
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoerrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// adxDataProducer uses the ADX client to perform ingestion
type adxDataProducer struct {
	client        *kusto.Client       // client for logs , traces and metrics
	ingestor      ingest.Ingestor     // ingestion for  logs, traces and metrics
	ingestOptions []ingest.FileOption // Options for the ingestion
	logger        *zap.Logger         // Loggers for tracing the flow
}

const nextline = "\n"

const (
	// Scope name
	scopename = "scope.name"
	// Scope version
	scopeversion = "scope.version"
)

// given the full metrics , extract each metric , resource attributes and scope attributes. Individual metric mapping is sent on to metricdata mapping
func (e *adxDataProducer) metricsDataPusher(ctx context.Context, metrics pmetric.Metrics) error {
	transformedAdxMetrics, err := rawMetricsToAdxMetrics(ctx, metrics, e.logger)
	metricsBuffer := make([]string, len(transformedAdxMetrics))
	if err != nil {
		e.logger.Error("Error transforming metrics to ADX metric format.", zap.Error(err))
		return err
	}
	// Since the transform succeeded ,  using the option for ingestion ingest the data into ADX
	for idx, tm := range transformedAdxMetrics {
		adxMetricJsonString, err := jsoniter.MarshalToString(tm)
		if err != nil {
			e.logger.Error("Error performing serialization of data.", zap.Error(err))
		}
		metricsBuffer[idx] = adxMetricJsonString
	}
	if len(metricsBuffer) != 0 {
		if err := e.ingestData(metricsBuffer); err != nil {
			return err
		}
	}
	metricsFlushed := len(transformedAdxMetrics)
	e.logger.Sugar().Infof("Flushing %d metrics to sink", metricsFlushed)
	return nil
}

func (e *adxDataProducer) ingestData(b []string) error {

	ingestReader := strings.NewReader(strings.Join(b, nextline))

	if _, err := e.ingestor.FromReader(context.Background(), ingestReader, e.ingestOptions...); err != nil {
		e.logger.Error("Error performing managed data ingestion.", zap.Error(err))
		return err
	}
	return nil
}

func (e *adxDataProducer) logsDataPusher(ctx context.Context, logData plog.Logs) error {
	resourceLogs := logData.ResourceLogs()
	var logsBuffer []string
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i)
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scope := scopeLogs.At(j)
			logs := scopeLogs.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logData := logs.At(k)
				transformedADXLog := mapToAdxLog(resource.Resource(), scope.Scope(), logData, e.logger)
				adxLogJsonBytes, err := jsoniter.MarshalToString(transformedADXLog)
				if err != nil {
					e.logger.Error("Error performing serialization of data.", zap.Error(err))
				}
				logsBuffer = append(logsBuffer, adxLogJsonBytes)
			}
		}
	}
	if len(logsBuffer) != 0 {
		if err := e.ingestData(logsBuffer); err != nil {
			return err
		}
	}
	return nil
}

func (e *adxDataProducer) tracesDataPusher(ctx context.Context, traceData ptrace.Traces) error {
	resourceSpans := traceData.ResourceSpans()
	var spanBuffer []string
	for i := 0; i < resourceSpans.Len(); i++ {
		resource := resourceSpans.At(i)
		scopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scope := scopeSpans.At(j)
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				spanData := spans.At(k)
				transformedADXTrace := mapToAdxTrace(resource.Resource(), scope.Scope(), spanData, e.logger)
				adxTraceJsonBytes, err := jsoniter.MarshalToString(transformedADXTrace)
				if err != nil {
					e.logger.Error("Error performing serialization of data.", zap.Error(err))
				}
				spanBuffer = append(spanBuffer, adxTraceJsonBytes)
			}
		}
	}
	if len(spanBuffer) != 0 {
		if err := e.ingestData(spanBuffer); err != nil {
			return err
		}
	}
	return nil
}

func (adp *adxDataProducer) Close(context.Context) error {

	var err error

	err = adp.ingestor.Close()
	err2 := adp.client.Close()
	if err == nil {
		err = err2
	} else {
		err = kustoerrors.GetCombinedError(err, err2)
	}
	if err != nil {
		adp.logger.Warn("Error closing connections", zap.Error(err))
	} else {
		adp.logger.Info("Closed Ingestor and Client")
	}
	return err
}

/*
Create an exporter. The exporter instantiates a client , creates the ingestor and then sends data through it
*/
func newExporter(config *Config, logger *zap.Logger, telemetryDataType int) (*adxDataProducer, error) {
	tableName, err := getTableName(config, telemetryDataType)
	if err != nil {
		return nil, err
	}
	metricClient, err := buildAdxClient(config)

	if err != nil {
		return nil, err
	}

	var ingestor ingest.Ingestor

	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	// Expect that this mapping is already existent
	if refOption := getMappingRef(config, telemetryDataType); refOption != nil {
		ingestOptions = append(ingestOptions, refOption)
	}
	// The exporter could be configured to run in either modes. Using managedstreaming or batched queueing
	if strings.ToLower(config.IngestionType) == managedIngestType {
		mi, err := createManagedStreamingIngestor(config, metricClient, tableName)
		if err != nil {
			return nil, err
		}
		ingestor = mi
		err = nil
	} else {
		qi, err := createQueuedIngestor(config, metricClient, tableName)
		if err != nil {
			return nil, err
		}
		ingestor = qi
		err = nil
	}
	return &adxDataProducer{
		client:        metricClient,
		ingestOptions: ingestOptions,
		ingestor:      ingestor,
		logger:        logger,
	}, nil
}

/**
Common functions that are used by all the 3 parts of OTEL , namely Traces , Logs and Metrics
*/

/* Fetchs the coresponding ingetionRef if the mapping is provided*/
func getMappingRef(config *Config, telemetryDataType int) ingest.FileOption {
	switch telemetryDataType {
	case metricsType:
		if !isEmpty(config.OTELMetricTableMapping) {
			return ingest.IngestionMappingRef(config.OTELMetricTableMapping, ingest.JSON)
		}
	case tracesType:
		if !isEmpty(config.OTELTraceTableMapping) {
			return ingest.IngestionMappingRef(config.OTELTraceTableMapping, ingest.JSON)
		}
	case logsType:
		if !isEmpty(config.OTELLogTableMapping) {
			return ingest.IngestionMappingRef(config.OTELLogTableMapping, ingest.JSON)
		}
	}
	return nil
}

func buildAdxClient(config *Config) (*kusto.Client, error) {
	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ApplicationId,
			config.ApplicationKey, config.TenantId),
	}
	client, err := kusto.New(config.ClusterUri, authorizer)
	return client, err
}

// Depending on the table , create seperate ingestors
func createManagedStreamingIngestor(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Managed, error) {
	ingestor, err := ingest.NewManaged(adxclient, config.Database, tablename)
	return ingestor, err
}

// A queued ingestor in case that is provided as the config option
func createQueuedIngestor(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Ingestion, error) {
	ingestor, err := ingest.New(adxclient, config.Database, tablename)
	return ingestor, err
}

func getScopeMap(sc pcommon.InstrumentationScope) map[string]interface{} {
	scopeMap := make(map[string]interface{}, 2)

	if sc.Name() != "" {
		scopeMap[scopename] = sc.Name()
	}
	if sc.Version() != "" {
		scopeMap[scopeversion] = sc.Version()
	}

	return scopeMap
}

func getTableName(config *Config, telemetrydatatype int) (string, error) {
	switch telemetrydatatype {
	case metricsType:
		return config.OTELMetricTable, nil
	case logsType:
		return config.OTELLogTable, nil
	case tracesType:
		return config.OTELTraceTable, nil
	}
	return "", errors.New("invalid telemetry datatype")
}
