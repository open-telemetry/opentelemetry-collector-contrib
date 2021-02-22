// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

type emfExporter struct {
	//Each (log group, log stream) keeps a separate Pusher because of each (log group, log stream) requires separate stream token.
	groupStreamToPusherMap map[string]map[string]Pusher
	svcStructuredLog       LogClient
	config                 configmodels.Exporter
	logger                 *zap.Logger

	metricTranslator metricTranslator

	pusherMapLock sync.Mutex
	retryCnt      int
	collectorID   string
}

// New func creates an EMF Exporter instance with data push callback func
func New(
	config configmodels.Exporter,
	params component.ExporterCreateParams,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("emf exporter config is nil")
	}

	logger := params.Logger
	expConfig := config.(*Config)
	expConfig.logger = logger

	// create AWS session
	awsConfig, session, err := GetAWSConfigSession(logger, &Conn{}, expConfig)
	if err != nil {
		return nil, err
	}

	// create CWLogs client with aws session config
	svcStructuredLog := NewCloudWatchLogsClient(logger, awsConfig, params.ApplicationStartInfo, session)
	collectorIdentifier, _ := uuid.NewRandom()

	expConfig.Validate()

	emfExporter := &emfExporter{
		svcStructuredLog: svcStructuredLog,
		config:           config,
		metricTranslator: newMetricTranslator(*expConfig),
		retryCnt:         *awsConfig.MaxRetries,
		logger:           logger,
		collectorID:      collectorIdentifier.String(),
	}
	emfExporter.groupStreamToPusherMap = map[string]map[string]Pusher{}

	return emfExporter, nil
}

// NewEmfExporter creates a new exporter using exporterhelper
func NewEmfExporter(
	config configmodels.Exporter,
	params component.ExporterCreateParams,
) (component.MetricsExporter, error) {

	exp, err := New(config, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		config,
		params.Logger,
		exp.(*emfExporter).pushMetricsData,
		exporterhelper.WithResourceToTelemetryConversion(config.(*Config).ResourceToTelemetrySettings),
		exporterhelper.WithShutdown(exp.(*emfExporter).Shutdown),
	)
}

func (emf *emfExporter) pushMetricsData(_ context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
	groupedMetrics := make(map[interface{}]*GroupedMetric)
	expConfig := emf.config.(*Config)
	defaultLogStream := fmt.Sprintf("otel-stream-%s", emf.collectorID)

	rms := md.ResourceMetrics()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		emf.metricTranslator.translateOTelToGroupedMetric(&rm, groupedMetrics, expConfig)
	}

	for _, groupedMetric := range groupedMetrics {
		cWMetric := translateGroupedMetricToCWMetric(groupedMetric, expConfig)
		putLogEvent := translateCWMetricToEMF(cWMetric)

		logGroup := groupedMetric.Metadata.LogGroup
		logStream := groupedMetric.Metadata.LogStream
		if logStream == "" {
			logStream = defaultLogStream
		}

		pusher := emf.getPusher(logGroup, logStream)
		if pusher != nil {
			returnError := pusher.AddLogEntry(putLogEvent)
			if returnError != nil {
				err = wrapErrorIfBadRequest(&returnError)
				return
			}
			returnError = pusher.ForceFlush()
			if returnError != nil {
				err = wrapErrorIfBadRequest(&returnError)
				return
			}
		}
	}

	return
}

func (emf *emfExporter) getPusher(logGroup, logStream string) Pusher {
	emf.pusherMapLock.Lock()
	defer emf.pusherMapLock.Unlock()

	var ok bool
	var streamToPusherMap map[string]Pusher
	if streamToPusherMap, ok = emf.groupStreamToPusherMap[logGroup]; !ok {
		streamToPusherMap = map[string]Pusher{}
		emf.groupStreamToPusherMap[logGroup] = streamToPusherMap
	}

	var pusher Pusher
	if pusher, ok = streamToPusherMap[logStream]; !ok {
		pusher = NewPusher(aws.String(logGroup), aws.String(logStream), emf.retryCnt, emf.svcStructuredLog, emf.logger)
		streamToPusherMap[logStream] = pusher
	}
	return pusher
}

func (emf *emfExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	exporterCtx := obsreport.ExporterContext(ctx, "emf.exporterFullName")

	_, err := emf.pushMetricsData(exporterCtx, md)
	return err
}

// Shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) Shutdown(ctx context.Context) error {
	emf.pusherMapLock.Lock()
	defer emf.pusherMapLock.Unlock()

	var err error
	for _, streamToPusherMap := range emf.groupStreamToPusherMap {
		for _, pusher := range streamToPusherMap {
			if pusher != nil {
				returnError := pusher.ForceFlush()
				if returnError != nil {
					err = wrapErrorIfBadRequest(&returnError)
				}
				if err != nil {
					emf.logger.Error("Error when gracefully shutting down emf_exporter. Skipping to next pusher.", zap.Error(err))
				}
			}
		}
	}

	return nil
}

// Start
func (emf *emfExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func wrapErrorIfBadRequest(err *error) error {
	_, ok := (*err).(awserr.RequestFailure)
	if ok && (*err).(awserr.RequestFailure).StatusCode() < 500 {
		return consumererror.Permanent(*err)
	}
	return *err
}
