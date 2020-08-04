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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/translator"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	defaultForceFlushInterval          = 60 * time.Second
	defaultForceFlushIntervalInSeconds = 60
)

type emfExporter struct {
	svcStructuredLog LogClient
	config           configmodels.Exporter
	logger           *zap.Logger

	pusherWG           sync.WaitGroup
	ForceFlushInterval time.Duration
	shutdownChan       chan bool
	retryCnt           int

	token string
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

	emfExporter := &emfExporter{
		svcStructuredLog:   svcStructuredLog,
		config:             config,
		ForceFlushInterval: defaultForceFlushInterval,
		retryCnt:           *awsConfig.MaxRetries,
		logger:             logger,
	}
	if config.(*Config).ForceFlushInterval > 0 {
		emfExporter.ForceFlushInterval = time.Duration(config.(*Config).ForceFlushInterval) * time.Second
	}
	emfExporter.shutdownChan = make(chan bool)

	return emfExporter, nil
}

func (emf *emfExporter) pushMetricsData(_ context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
	expConfig := emf.config.(*Config)
	logGroup := "/metrics/default"
	logStream := "otel-stream"
	// override log group if customer has specified Resource Attributes service.name or service.namespace
	putLogEvents := generateLogEventFromMetric(md)

	// override log group if found it in exp configuration, this configuration has top priority.
	if len(expConfig.LogGroupName) > 0 {
		logGroup = expConfig.LogGroupName
	}
	if len(expConfig.LogStreamName) > 0 {
		logStream = expConfig.LogStreamName
	}
	if emf.token == "" {
		emf.token, _ = emf.svcStructuredLog.CreateStream(aws.String(logGroup), aws.String(logStream))
	}
	putLogEventsInput := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     putLogEvents,
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
	}
	if emf.token != "" {
		putLogEventsInput.SequenceToken = aws.String(emf.token)
	}
	tmpToken := emf.svcStructuredLog.PutLogEvents(putLogEventsInput, emf.retryCnt)
	if tmpToken != nil {
		emf.token = *tmpToken
	}

	return 0, nil
}

func (emf *emfExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	exporterCtx := obsreport.ExporterContext(ctx, "emf.exporterFullName")

	_, err := emf.pushMetricsData(exporterCtx, md)
	return err
}

// Shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) Shutdown(ctx context.Context) error {
	close(emf.shutdownChan)
	emf.pusherWG.Wait()
	return nil
}

// Start
func (emf *emfExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func generateLogEventFromMetric(metric pdata.Metrics) []*cloudwatchlogs.InputLogEvent {
	imd := pdatautil.MetricsToInternalMetrics(metric)
	rms := imd.ResourceMetrics()
	cwMetricLists := []*translator.CWMetrics{}
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if rm.IsNil() {
			continue
		}
		// translate OT metric datapoints into CWMetricLists
		cwm, err := translator.TranslateOtToCWMetric(&rm)
		if err != nil || cwm == nil {
			return nil
		}
		// append all datapoint metrics in the request into CWMetric list
		cwMetricLists = append(cwMetricLists, cwm...)
	}

	return translator.TranslateCWMetricToEMF(cwMetricLists)
}
