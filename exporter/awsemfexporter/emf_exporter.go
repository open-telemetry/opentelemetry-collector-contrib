// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

const (
	// OutputDestination Options
	outputDestinationCloudWatch = "cloudwatch"
	outputDestinationStdout     = "stdout"

	// AppSignals EMF config
	appSignalsMetricNamespace    = "ApplicationSignals"
	appSignalsLogGroupNamePrefix = "/aws/application-signals/"
)

type emfExporter struct {
	pusherMap        map[cwlogs.StreamKey]cwlogs.Pusher
	svcStructuredLog *cwlogs.Client
	config           *Config

	metricTranslator metricTranslator

	pusherMapLock sync.Mutex
	retryCnt      int
	collectorID   string
}

// newEmfExporter creates a new exporter using exporterhelper
func newEmfExporter(config *Config, set exporter.Settings) (*emfExporter, error) {
	if config == nil {
		return nil, errors.New("emf exporter config is nil")
	}

	config.logger = set.Logger

	// create AWS session
	awsConfig, session, err := awsutil.GetAWSConfigSession(set.Logger, &awsutil.Conn{}, &config.AWSSessionSettings)
	if err != nil {
		return nil, err
	}

	var userAgentExtras []string
	if config.isAppSignalsEnabled() {
		userAgentExtras = append(userAgentExtras, "AppSignals")
	}

	// create CWLogs client with aws session config
	svcStructuredLog := cwlogs.NewClient(set.Logger,
		awsConfig,
		set.BuildInfo,
		config.LogGroupName,
		config.LogRetention,
		config.Tags,
		session,
		metadata.Type.String(),
		cwlogs.WithUserAgentExtras(userAgentExtras...),
	)
	collectorIdentifier, err := uuid.NewRandom()

	if err != nil {
		return nil, err
	}

	emfExporter := &emfExporter{
		svcStructuredLog: svcStructuredLog,
		config:           config,
		metricTranslator: newMetricTranslator(*config),
		retryCnt:         *awsConfig.MaxRetries,
		collectorID:      collectorIdentifier.String(),
		pusherMap:        map[cwlogs.StreamKey]cwlogs.Pusher{},
	}

	config.logger.Warn("the default value for DimensionRollupOption will be changing to NoDimensionRollup" +
		"in a future release. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/23997 for more" +
		"information")

	return emfExporter, nil
}

func (emf *emfExporter) pushMetricsData(_ context.Context, md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	labels := map[string]string{}
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		am := rm.Resource().Attributes()
		if am.Len() > 0 {
			am.Range(func(k string, v pcommon.Value) bool {
				labels[k] = v.Str()
				return true
			})
		}
	}
	emf.config.logger.Debug("Start processing resource metrics", zap.Any("labels", labels))

	groupedMetrics := make(map[any]*groupedMetric)
	defaultLogStream := fmt.Sprintf("otel-stream-%s", emf.collectorID)
	outputDestination := emf.config.OutputDestination

	for i := 0; i < rms.Len(); i++ {
		err := emf.metricTranslator.translateOTelToGroupedMetric(rms.At(i), groupedMetrics, emf.config)
		if err != nil {
			return err
		}
	}

	for _, groupedMetric := range groupedMetrics {
		putLogEvent, err := translateGroupedMetricToEmf(groupedMetric, emf.config, defaultLogStream)
		if err != nil {
			return err
		}
		// Currently we only support two options for "OutputDestination".
		if strings.EqualFold(outputDestination, outputDestinationStdout) {
			if putLogEvent != nil &&
				putLogEvent.InputLogEvent != nil &&
				putLogEvent.InputLogEvent.Message != nil {
				fmt.Println(*putLogEvent.InputLogEvent.Message)
			}
		} else if strings.EqualFold(outputDestination, outputDestinationCloudWatch) {

			emfPusher := emf.getPusher(putLogEvent.StreamKey)
			if emfPusher != nil {
				returnError := emfPusher.AddLogEntry(putLogEvent)
				if returnError != nil {
					return wrapErrorIfBadRequest(returnError)
				}
			}
		}
	}

	if strings.EqualFold(outputDestination, outputDestinationCloudWatch) {
		for _, emfPusher := range emf.listPushers() {
			returnError := emfPusher.ForceFlush()
			if returnError != nil {
				// TODO now we only have one logPusher, so it's ok to return after first error occurred
				err := wrapErrorIfBadRequest(returnError)
				if err != nil {
					emf.config.logger.Error("Error force flushing logs. Skipping to next logPusher.", zap.Error(err))
				}
				return err
			}
		}
	}

	emf.config.logger.Debug("Finish processing resource metrics", zap.Any("labels", labels))

	return nil
}

func (emf *emfExporter) getPusher(key cwlogs.StreamKey) cwlogs.Pusher {

	var ok bool
	if _, ok = emf.pusherMap[key]; !ok {
		emf.pusherMap[key] = cwlogs.NewPusher(key, emf.retryCnt, *emf.svcStructuredLog, emf.config.logger)
	}
	return emf.pusherMap[key]
}

func (emf *emfExporter) listPushers() []cwlogs.Pusher {
	emf.pusherMapLock.Lock()
	defer emf.pusherMapLock.Unlock()

	var pushers []cwlogs.Pusher
	for _, pusher := range emf.pusherMap {
		pushers = append(pushers, pusher)
	}
	return pushers
}

// shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) shutdown(_ context.Context) error {
	for _, emfPusher := range emf.listPushers() {
		returnError := emfPusher.ForceFlush()
		if returnError != nil {
			err := wrapErrorIfBadRequest(returnError)
			if err != nil {
				emf.config.logger.Error("Error when gracefully shutting down emf_exporter. Skipping to next logPusher.", zap.Error(err))
			}
		}
	}

	return emf.metricTranslator.Shutdown()
}

func wrapErrorIfBadRequest(err error) error {
	var rfErr awserr.RequestFailure
	if errors.As(err, &rfErr) && rfErr.StatusCode() < 500 {
		return consumererror.NewPermanent(err)
	}
	return err
}
