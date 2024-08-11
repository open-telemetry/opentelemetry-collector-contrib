// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	transport              = "http"
	dataFormat             = "cloudfoundry"
	instrumentationLibName = "otelcol/cloudfoundry"
)

var _ receiver.Metrics = (*cloudFoundryReceiver)(nil)
var _ receiver.Logs = (*cloudFoundryReceiver)(nil)

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
	nextConsumer consumer.Metrics) (*cloudFoundryReceiver, error) {

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
	nextConsumer consumer.Logs) (*cloudFoundryReceiver, error) {

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
	cfr.goroutines.Add(1)

	go func() {
		defer cfr.goroutines.Done()
		cfr.settings.Logger.Debug("cloudfoundry receiver starting")
		_, tokenErr = tokenProvider.ProvideToken()
		if tokenErr != nil {
			cfr.settings.ReportStatus(
				component.NewFatalErrorEvent(
					fmt.Errorf("cloudfoundry receiver failed to fetch initial token from UAA: %w", tokenErr),
				),
			)
			return
		}
		if cfr.nextLogs != nil {
			cfr.streamLogs(innerCtx, streamFactory.CreateLogsStream(innerCtx, cfr.config.RLPGateway.ShardID))
		} else if cfr.nextMetrics != nil {
			cfr.streamMetrics(innerCtx, streamFactory.CreateMetricsStream(innerCtx, cfr.config.RLPGateway.ShardID))
		}
		cfr.settings.Logger.Debug("cloudfoundry receiver stopped")
	}()
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
	stream loggregator.EnvelopeStream) {

	for {
		// Blocks until non-empty result or context is cancelled (returns nil in that case)
		envelopes := stream()
		if envelopes == nil {
			// If context has not been cancelled, then nil means the shutdown was due to an error within stream
			if ctx.Err() == nil {
				cfr.settings.ReportStatus(
					component.NewFatalErrorEvent(
						errors.New("RLP gateway metrics streamer shut down due to an error"),
					),
				)
			}
			break
		}
		metrics := pmetric.NewMetrics()
		libraryMetrics := createLibraryMetricsSlice(metrics)
		for _, envelope := range envelopes {
			if envelope != nil {
				// There is no concept of startTime in CF loggregator, and we do not know the uptime of the component
				// from which the metric originates, so just provide receiver start time as metric start time
				convertEnvelopeToMetrics(envelope, libraryMetrics, cfr.receiverStartTime)
			}
		}
		if libraryMetrics.Len() > 0 {
			obsCtx := cfr.obsrecv.StartMetricsOp(ctx)
			err := cfr.nextMetrics.ConsumeMetrics(ctx, metrics)
			cfr.obsrecv.EndMetricsOp(obsCtx, dataFormat, metrics.DataPointCount(), err)
		}
	}
}

func (cfr *cloudFoundryReceiver) streamLogs(
	ctx context.Context,
	stream loggregator.EnvelopeStream) {

	for {
		envelopes := stream()
		if envelopes == nil {
			if ctx.Err() == nil {
				cfr.settings.ReportStatus(
					component.NewFatalErrorEvent(
						errors.New("RLP gateway log streamer shut down due to an error"),
					),
				)
			}
			break
		}
		logs := plog.NewLogs()
		libraryLogs := createLibraryLogsSlice(logs)
		observedTime := time.Now()
		for _, envelope := range envelopes {
			if envelope != nil {
				_ = convertEnvelopeToLogs(envelope, libraryLogs, observedTime)
			}
		}
		if libraryLogs.Len() > 0 {
			obsCtx := cfr.obsrecv.StartLogsOp(ctx)
			err := cfr.nextLogs.ConsumeLogs(ctx, logs)
			cfr.obsrecv.EndLogsOp(obsCtx, dataFormat, logs.LogRecordCount(), err)
		}
	}
}

func createLibraryMetricsSlice(metrics pmetric.Metrics) pmetric.MetricSlice {
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()
	libraryMetrics := resourceMetric.ScopeMetrics().AppendEmpty()
	libraryMetrics.Scope().SetName(instrumentationLibName)
	return libraryMetrics.Metrics()
}

func createLibraryLogsSlice(logs plog.Logs) plog.LogRecordSlice {
	resourceLog := logs.ResourceLogs().AppendEmpty()
	libraryLogs := resourceLog.ScopeLogs().AppendEmpty()
	libraryLogs.Scope().SetName(instrumentationLibName)
	return libraryLogs.LogRecords()
}
