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
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

type emfExporter struct {
	//Each (log group, log stream) keeps a separate Pusher because of each (log group, log stream) requires separate stream token.
	groupStreamToPusherMap map[string]map[string]Pusher
	svcStructuredLog       LogClient
	config                 configmodels.Exporter
	logger                 *zap.Logger

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
	// create AWS session
	awsConfig, session, err := GetAWSConfigSession(logger, &Conn{}, config.(*Config))
	if err != nil {
		return nil, err
	}

	// create CWLogs client with aws session config
	svcStructuredLog := NewCloudWatchLogsClient(logger, awsConfig, session)
	collectorIdentifier, _ := uuid.NewRandom()

	emfExporter := &emfExporter{
		svcStructuredLog: svcStructuredLog,
		config:           config,
		retryCnt:         *awsConfig.MaxRetries,
		logger:           logger,
		collectorID:      collectorIdentifier.String(),
	}
	emfExporter.groupStreamToPusherMap = map[string]map[string]Pusher{}

	return emfExporter, nil
}

func (emf *emfExporter) pushMetricsData(_ context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
	expConfig := emf.config.(*Config)
	dimensionRollupOption := expConfig.DimensionRollupOption
	logGroup := "/metrics/default"
	logStream := fmt.Sprintf("otel-stream-%s", emf.collectorID)
	// override log group if customer has specified Resource Attributes service.name or service.namespace
	putLogEvents, totalDroppedMetrics, namespace := generateLogEventFromMetric(md, dimensionRollupOption, expConfig.Namespace)
	if namespace != "" {
		logGroup = fmt.Sprintf("/metrics/%s", namespace)
	}
	// override log group if found it in exp configuration, this configuration has top priority. However, in this case, customer won't have correlation experience

	if len(expConfig.LogGroupName) > 0 {
		logGroup = expConfig.LogGroupName
	}
	if len(expConfig.LogStreamName) > 0 {
		logStream = expConfig.LogStreamName
	}
	pusher := emf.getPusher(logGroup, logStream)
	if pusher != nil {
		for _, ple := range putLogEvents {
			returnError := pusher.AddLogEntry(ple)
			if returnError != nil {
				err = wrapErrorIfBadRequest(&returnError)
			}
			if err != nil {
				return totalDroppedMetrics, err
			}
		}
		returnError := pusher.ForceFlush()
		if returnError != nil {
			err = wrapErrorIfBadRequest(&returnError)
		}
		if err != nil {
			return totalDroppedMetrics, err
		}
	}
	return totalDroppedMetrics, nil
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

func generateLogEventFromMetric(metric pdata.Metrics, dimensionRollupOption string, namespace string) ([]*LogEvent, int, string) {
	rms := metric.ResourceMetrics()
	cwMetricLists := []*CWMetrics{}
	var cwm []*CWMetrics
	var totalDroppedMetrics int

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if rm.IsNil() {
			continue
		}
		cwm, totalDroppedMetrics = TranslateOtToCWMetric(&rm, dimensionRollupOption, namespace)
		if len(cwm) > 0 && len(cwm[0].Measurements) > 0 {
			namespace = cwm[0].Measurements[0].Namespace
		}
		// append all datapoint metrics in the request into CWMetric list
		cwMetricLists = append(cwMetricLists, cwm...)
	}

	return TranslateCWMetricToEMF(cwMetricLists), totalDroppedMetrics, namespace
}

func wrapErrorIfBadRequest(err *error) error {
	_, ok := (*err).(awserr.RequestFailure)
	if ok && (*err).(awserr.RequestFailure).StatusCode() < 500 {
		return consumererror.Permanent(*err)
	}
	return *err
}
