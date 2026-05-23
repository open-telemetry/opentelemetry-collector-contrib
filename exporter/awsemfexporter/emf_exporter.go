// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/useragent"
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

var enhancedContainerInsightsEKSPattern = regexp.MustCompile(`^/aws/containerinsights/\S+/performance$`)

type emfExporter struct {
	pusherMap        sync.Map
	svcStructuredLog *cwlogs.Client
	config           *Config
	set              exporter.Settings

	metricTranslator metricTranslator

	retryCnt    int
	collectorID string

	processResourceLabels func(map[string]string)
	processMetrics        func(pmetric.Metrics)
}

// newEmfExporter creates a new exporter using exporterhelper
func newEmfExporter(_ context.Context, config *Config, set exporter.Settings) (*emfExporter, error) {
	if config == nil {
		return nil, errors.New("emf exporter config is nil")
	}

	config.logger = set.Logger

	collectorIdentifier, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	emfExporter := &emfExporter{
		config:                config,
		set:                   set,
		metricTranslator:      newMetricTranslator(*config),
		retryCnt:              config.MaxRetries,
		collectorID:           collectorIdentifier.String(),
		processResourceLabels: func(map[string]string) {},
		processMetrics:        func(pmetric.Metrics) {},
	}

	config.logger.Warn("the default value for DimensionRollupOption will be changing to NoDimensionRollup" +
		"in a future release. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/23997 for more" +
		"information")

	return emfExporter, nil
}

func (emf *emfExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	labels := map[string]string{}
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		am := rm.Resource().Attributes()
		if am.Len() > 0 {
			for k, v := range am.All() {
				labels[k] = v.Str()
			}
		}
	}
	emf.config.logger.Debug("Start processing resource metrics", zap.Any("labels", labels))
	emf.processResourceLabels(labels)
	emf.processMetrics(md)

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
				putLogEvent.InputLogEvent.Message != nil {
				fmt.Println(*putLogEvent.InputLogEvent.Message)
			}
		} else if strings.EqualFold(outputDestination, outputDestinationCloudWatch) {
			emfPusher, err := emf.getPusher(putLogEvent.StreamKey)
			if err != nil {
				return fmt.Errorf("failed to get pusher: %w", err)
			}
			if emfPusher != nil {
				returnError := emfPusher.AddLogEntry(ctx, putLogEvent)
				if returnError != nil {
					return wrapErrorIfBadRequest(returnError)
				}
			}
		}
	}

	if strings.EqualFold(outputDestination, outputDestinationCloudWatch) {
		for _, emfPusher := range emf.listPushers() {
			returnError := emfPusher.ForceFlush(ctx)
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
	if emf.svcStructuredLog == nil {
		return nil, errors.New("CloudWatch Logs client not initialized")
	}

	if p, ok := emf.pusherMap.Load(key); ok {
		return p.(cwlogs.Pusher), nil
	}

	logger := emf.set.Logger
	if logger == nil {
		logger = emf.config.logger
	}
	p, _ := emf.pusherMap.LoadOrStore(key, cwlogs.NewPusher(key, emf.retryCnt, *emf.svcStructuredLog, logger))
	return p.(cwlogs.Pusher), nil
}

func (emf *emfExporter) listPushers() []cwlogs.Pusher {
	var pushers []cwlogs.Pusher
	emf.pusherMap.Range(func(_, value any) bool {
		pushers = append(pushers, value.(cwlogs.Pusher))
		return true
	})
	return pushers
}

func (emf *emfExporter) start(ctx context.Context, host component.Host) error {
	awsConfig, err := awsutil.GetAWSConfig(ctx, emf.config.logger, &emf.config.AWSSessionSettings)
	if err != nil {
		return err
	}

	var userAgentExtras []string
	if emf.config.IsAppSignalsEnabled() {
		userAgentExtras = append(userAgentExtras, "AppSignals")
	}
	if emf.config.IsEnhancedContainerInsights() && enhancedContainerInsightsEKSPattern.MatchString(emf.config.LogGroupName) {
		userAgentExtras = append(userAgentExtras, "EnhancedEKSContainerInsights")
	}

	// Wire dynamic user-agent middleware before NewClient — APIOptions are
	// snapshotted at client construction time.
	var ua *useragent.UserAgent
	if emf.config.IsAppSignalsEnabled() || emf.config.IsEnhancedContainerInsights() {
		ua = useragent.NewUserAgent()
		awsConfig.APIOptions = append(awsConfig.APIOptions, ua.APIOption())
	}

	if emf.config.MiddlewareID != nil {
		awsmiddleware.TryConfigure(emf.config.logger, host, *emf.config.MiddlewareID, awsmiddleware.SDKv2(&awsConfig))
	}

	emf.svcStructuredLog = cwlogs.NewClient(
		emf.config.logger,
		awsConfig,
		emf.set.BuildInfo,
		emf.config.LogGroupName,
		emf.config.LogRetention,
		emf.config.Tags,
		metadata.Type.String(),
		cwlogs.WithUserAgentExtras(userAgentExtras...),
	)

	if ua != nil {
		if emf.config.IsAppSignalsEnabled() {
			emf.processResourceLabels = ua.Process
		}
		if emf.config.IsEnhancedContainerInsights() {
			emf.processMetrics = ua.ProcessMetrics
		}
	}

	return nil
}

// shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) shutdown(ctx context.Context) error {
	for _, emfPusher := range emf.listPushers() {
		returnError := emfPusher.ForceFlush(ctx)
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
	var ae smithy.APIError
	if errors.As(err, &ae) {
		if ae.ErrorFault() == smithy.FaultClient || ae.ErrorFault() == smithy.FaultUnknown {
			return consumererror.NewPermanent(err)
		}
	}
	return err
}
