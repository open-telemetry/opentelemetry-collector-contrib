// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/internal/metadata"
)

const (
	transport  = "http"
	dataFormat = "cloudfoundry"
)

var (
	_ receiver.Metrics = (*cloudFoundryReceiver)(nil)
	_ receiver.Logs    = (*cloudFoundryReceiver)(nil)
)

// newCloudFoundryReceiver implements the receiver.Metrics and receiver.Logs for the Cloud Foundry protocol.
type cloudFoundryReceiver struct {
	settings          component.TelemetrySettings
	cancel            context.CancelFunc
	config            Config
	nextMetrics       consumer.Metrics
	nextLogs          consumer.Logs
	obsrecv           *receiverhelper.ObsReport
	goroutines        sync.WaitGroup
	receiverStartTime time.Time
}

// newCloudFoundryMetricsReceiver creates the Cloud Foundry receiver with the given parameters.
func newCloudFoundryMetricsReceiver(
	settings receiver.Settings,
	config Config,
	nextConsumer consumer.Metrics,
) (*cloudFoundryReceiver, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	result := &cloudFoundryReceiver{
		settings:          settings.TelemetrySettings,
		config:            config,
		nextMetrics:       nextConsumer,
		obsrecv:           obsrecv,
		receiverStartTime: time.Now(),
	}
	return result, nil
}

// newCloudFoundryLogsReceiver creates the Cloud Foundry logs receiver with the given parameters.
func newCloudFoundryLogsReceiver(
	settings receiver.Settings,
	config Config,
	nextConsumer consumer.Logs,
) (*cloudFoundryReceiver, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	result := &cloudFoundryReceiver{
		settings:          settings.TelemetrySettings,
		config:            config,
		nextLogs:          nextConsumer,
		obsrecv:           obsrecv,
		receiverStartTime: time.Now(),
	}
	return result, nil
}

func (cfr *cloudFoundryReceiver) Start(ctx context.Context, host component.Host) error {
	tokenProvider, tokenErr := newUAATokenProvider(
		cfr.settings.Logger,
		cfr.config.UAA.LimitedClientConfig,
		cfr.config.UAA.Username,
		string(cfr.config.UAA.Password),
	)
	if tokenErr != nil {
		return fmt.Errorf("cloudfoundry receiver failed to create UAA token provider: %w", tokenErr)
	}
	streamFactory, streamErr := newEnvelopeStreamFactory(
		ctx,
		cfr.settings,
		tokenProvider,
		cfr.config.RLPGateway.ClientConfig,
		host,
	)
	if streamErr != nil {
		return fmt.Errorf("cloudfoundry receiver failed to create RLP envelope stream factory: %w", streamErr)
	}

	innerCtx, cancel := context.WithCancel(ctx)
	cfr.cancel = cancel
	cfr.goroutines.Go(func() {
		cfr.settings.Logger.Debug("cloudfoundry receiver starting")
		_, tokenErr = tokenProvider.ProvideToken()
		if tokenErr != nil {
			componentstatus.ReportStatus(
				host,
				componentstatus.NewFatalErrorEvent(
					fmt.Errorf("cloudfoundry receiver failed to fetch initial token from UAA: %w", tokenErr),
				),
			)
			return
		}
		if cfr.nextLogs != nil {
			cfr.streamLogs(innerCtx, streamFactory.CreateLogsStream(innerCtx, cfr.config.RLPGateway.ShardID), host)
		} else if cfr.nextMetrics != nil {
			cfr.streamMetrics(innerCtx, streamFactory.CreateMetricsStream(innerCtx, cfr.config.RLPGateway.ShardID), host)
		}
		cfr.settings.Logger.Debug("cloudfoundry receiver stopped")
	})
	return nil
}

func (cfr *cloudFoundryReceiver) Shutdown(_ context.Context) error {
	if cfr.cancel == nil {
		return nil
	}
	cfr.cancel()
	cfr.goroutines.Wait()
	return nil
}

func (cfr *cloudFoundryReceiver) streamMetrics(
	ctx context.Context,
	stream loggregator.EnvelopeStream,
	host component.Host,
) {
	for {
		// Blocks until non-empty result or context is cancelled (returns nil in that case)
		envelopes := stream()
		if envelopes == nil {
			// If context has not been cancelled, then nil means the shutdown was due to an error within stream
			if ctx.Err() == nil {
				componentstatus.ReportStatus(
					host,
					componentstatus.NewFatalErrorEvent(
						errors.New("RLP gateway metrics streamer shut down due to an error"),
					),
				)
			}
			break
		}
		metrics := pmetric.NewMetrics()
		for _, envelope := range envelopes {
			if envelope != nil {
				buildMetrics(metrics, envelope, cfr.receiverStartTime)
			}
		}
		if metrics.ResourceMetrics().Len() > 0 {
			obsCtx := cfr.obsrecv.StartMetricsOp(ctx)
			err := cfr.nextMetrics.ConsumeMetrics(ctx, metrics)
			if err != nil {
				cfr.settings.Logger.Error("Failed to consume metrics", zap.Error(err))
			}
			cfr.obsrecv.EndMetricsOp(obsCtx, dataFormat, metrics.DataPointCount(), err)
		}
	}
}

func (cfr *cloudFoundryReceiver) streamLogs(
	ctx context.Context,
	stream loggregator.EnvelopeStream,
	host component.Host,
) {
	for {
		envelopes := stream()
		if envelopes == nil {
			if ctx.Err() == nil {
				componentstatus.ReportStatus(
					host,
					componentstatus.NewFatalErrorEvent(
						errors.New("RLP gateway log streamer shut down due to an error"),
					),
				)
			}
			break
		}
		logs := plog.NewLogs()
		observedTime := time.Now()
		for _, envelope := range envelopes {
			if envelope != nil {
				buildLogs(logs, envelope, observedTime)
			}
		}
		if logs.ResourceLogs().Len() > 0 {
			obsCtx := cfr.obsrecv.StartLogsOp(ctx)
			err := cfr.nextLogs.ConsumeLogs(ctx, logs)
			if err != nil {
				cfr.settings.Logger.Error("Failed to consume logs", zap.Error(err))
			}
			cfr.obsrecv.EndLogsOp(obsCtx, dataFormat, logs.LogRecordCount(), err)
		}
	}
}

func buildLogs(logs plog.Logs, envelope *loggregator_v2.Envelope, observedTime time.Time) {
	resourceLogs := getResourceLogs(logs, envelope)
	setupLogsScope(resourceLogs)
	_ = convertEnvelopeToLogs(envelope, resourceLogs.ScopeLogs().At(0).LogRecords(), observedTime)
}

func buildMetrics(metrics pmetric.Metrics, envelope *loggregator_v2.Envelope, observedTime time.Time) {
	resourceMetrics := getResourceMetrics(metrics, envelope)
	setupMetricsScope(resourceMetrics)
	convertEnvelopeToMetrics(envelope, resourceMetrics.ScopeMetrics().At(0).Metrics(), observedTime)
}

func setupMetricsScope(resourceMetrics pmetric.ResourceMetrics) {
	if resourceMetrics.ScopeMetrics().Len() == 0 {
		libraryMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		libraryMetrics.Scope().SetName(metadata.ScopeName)
	}
}

func getResourceMetrics(metrics pmetric.Metrics, envelope *loggregator_v2.Envelope) pmetric.ResourceMetrics {
	if !allowResourceAttributes.IsEnabled() {
		return metrics.ResourceMetrics().AppendEmpty()
	}

	attrs := getEnvelopeResourceAttributes(envelope)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		if reflect.DeepEqual(metrics.ResourceMetrics().At(i).Resource().Attributes().AsRaw(), attrs.AsRaw()) {
			return metrics.ResourceMetrics().At(i)
		}
	}
	resource := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(resource.Resource().Attributes())
	return resource
}

func setupLogsScope(resourceLogs plog.ResourceLogs) {
	if resourceLogs.ScopeLogs().Len() == 0 {
		libraryLogs := resourceLogs.ScopeLogs().AppendEmpty()
		libraryLogs.Scope().SetName(metadata.ScopeName)
	}
}

func getResourceLogs(logs plog.Logs, envelope *loggregator_v2.Envelope) plog.ResourceLogs {
	if !allowResourceAttributes.IsEnabled() {
		return logs.ResourceLogs().AppendEmpty()
	}

	attrs := getEnvelopeResourceAttributes(envelope)
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		if reflect.DeepEqual(logs.ResourceLogs().At(i).Resource().Attributes().AsRaw(), attrs.AsRaw()) {
			return logs.ResourceLogs().At(i)
		}
	}
	resource := logs.ResourceLogs().AppendEmpty()
	attrs.CopyTo(resource.Resource().Attributes())
	return resource
}
