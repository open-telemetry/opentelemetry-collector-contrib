// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/appsignals"
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
	set              exporter.Settings

	metricTranslator metricTranslator

	pusherMapLock sync.Mutex
	retryCnt      int
	collectorID   string

	processResourceLabels func(map[string]string)
}

// newEmfExporter creates a new exporter using exporterhelper
func newEmfExporter(config *Config, set exporter.Settings) (*emfExporter, error) {
	if config == nil {
		return nil, errors.New("emf exporter config is nil")
	}

	config.logger = set.Logger

	collectorIdentifier, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	// Initialize emfExporter without AWS session and structured logs
	emfExporter := &emfExporter{
		config:                config,
		metricTranslator:      newMetricTranslator(*config),
		retryCnt:              config.AWSSessionSettings.MaxRetries,
		collectorID:           collectorIdentifier.String(),
		pusherMap:             map[cwlogs.StreamKey]cwlogs.Pusher{},
		processResourceLabels: func(map[string]string) {},
	}

	if config.IsAppSignalsEnabled() {
		userAgent := appsignals.NewUserAgent()
		emfExporter.processResourceLabels = userAgent.Process
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
	emf.processResourceLabels(labels)

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
			if errors.Is(err, errMissingMetricsForEnhancedContainerInsights) {
				emf.config.logger.Debug("Dropping empty putLogEvents for enhanced container insights", zap.Error(err))
				continue
			}
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
			emfPusher, err := emf.getPusher(putLogEvent.StreamKey)
			if err != nil {
				return fmt.Errorf("failed to get pusher: %w", err)
			}
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

func (emf *emfExporter) getPusher(key cwlogs.StreamKey) (cwlogs.Pusher, error) {
	emf.pusherMapLock.Lock()
	defer emf.pusherMapLock.Unlock()

	if emf.svcStructuredLog == nil {
		return nil, errors.New("CloudWatch Logs client not initialized")
	}

	pusher, exists := emf.pusherMap[key]
	if !exists {
		if emf.set.Logger != nil {
			pusher = cwlogs.NewPusher(key, emf.retryCnt, *emf.svcStructuredLog, emf.set.Logger)
		} else {
			pusher = cwlogs.NewPusher(key, emf.retryCnt, *emf.svcStructuredLog, emf.config.logger)
		}
		emf.pusherMap[key] = pusher
	}
	return pusher, nil
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

func (emf *emfExporter) start(_ context.Context, host component.Host) error {
	// Create AWS session here
	awsConfig, session, err := awsutil.GetAWSConfigSession(emf.config.logger, &awsutil.Conn{}, &emf.config.AWSSessionSettings)
	if err != nil {
		return err
	}

	// create CWLogs client with aws session config
	svcStructuredLog := cwlogs.NewClient(emf.config.logger,
		awsConfig,
		emf.set.BuildInfo,
		emf.config.LogGroupName,
		emf.config.LogRetention,
		emf.config.Tags,
		session,
		cwlogs.WithEnabledContainerInsights(emf.config.IsEnhancedContainerInsights()),
		cwlogs.WithEnabledAppSignals(emf.config.IsAppSignalsEnabled()),
	)

	// Assign to the struct
	emf.svcStructuredLog = svcStructuredLog

	// Optionally configure middleware
	if emf.config.MiddlewareID != nil {
		awsmiddleware.TryConfigure(emf.config.logger, host, *emf.config.MiddlewareID, awsmiddleware.SDKv1(svcStructuredLog.Handlers()))
	}

	return nil
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
