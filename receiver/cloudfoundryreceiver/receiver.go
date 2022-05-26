// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	transport              = "http"
	dataFormat             = "cloudfoundry"
	instrumentationLibName = "otelcol/cloudfoundry"
)

var _ component.MetricsReceiver = (*cloudFoundryReceiver)(nil)

// newCloudFoundryReceiver implements the component.MetricsReceiver for Cloud Foundry protocol.
type cloudFoundryReceiver struct {
	settings          component.TelemetrySettings
	cancel            context.CancelFunc
	config            Config
	nextConsumer      consumer.Metrics
	obsrecv           *obsreport.Receiver
	goroutines        sync.WaitGroup
	receiverStartTime time.Time
}

// newCloudFoundryReceiver creates the Cloud Foundry receiver with the given parameters.
func newCloudFoundryReceiver(
	settings component.ReceiverCreateSettings,
	config Config,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	return &cloudFoundryReceiver{
		settings:     settings.TelemetrySettings,
		config:       config,
		nextConsumer: nextConsumer,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              transport,
			ReceiverCreateSettings: settings,
		}),
		receiverStartTime: time.Now(),
	}, nil
}

func (cfr *cloudFoundryReceiver) Start(ctx context.Context, host component.Host) error {
	tokenProvider, tokenErr := newUAATokenProvider(cfr.settings.Logger, cfr.config.UAA.LimitedHTTPClientSettings, cfr.config.UAA.Username, cfr.config.UAA.Password)
	if tokenErr != nil {
		return fmt.Errorf("create cloud foundry UAA token provider: %v", tokenErr)
	}

	streamFactory, streamErr := newEnvelopeStreamFactory(
		cfr.settings,
		tokenProvider,
		cfr.config.RLPGateway.HTTPClientSettings,
		host,
	)
	if streamErr != nil {
		return fmt.Errorf("creating cloud foundry RLP envelope stream factory: %v", streamErr)
	}

	innerCtx, cancel := context.WithCancel(context.Background())
	cfr.cancel = cancel

	cfr.goroutines.Add(1)

	go func() {
		defer cfr.goroutines.Done()
		cfr.settings.Logger.Debug("cloud foundry receiver starting")

		_, tokenErr = tokenProvider.ProvideToken()
		if tokenErr != nil {
			host.ReportFatalError(fmt.Errorf("cloud foundry receiver failed to fetch initial token from UAA: %v", tokenErr))
			return
		}

		envelopeStream, err := streamFactory.CreateStream(innerCtx, cfr.config.RLPGateway.ShardID)
		if err != nil {
			host.ReportFatalError(fmt.Errorf("creating RLP gateway envelope stream: %v", err))
			return
		}

		cfr.streamMetrics(innerCtx, envelopeStream, host)
		cfr.settings.Logger.Debug("cloudfoundry metrics streamer stopped")
	}()

	return nil
}

func (cfr *cloudFoundryReceiver) Shutdown(_ context.Context) error {
	cfr.cancel()
	cfr.goroutines.Wait()
	return nil
}

func (cfr *cloudFoundryReceiver) streamMetrics(
	ctx context.Context,
	stream loggregator.EnvelopeStream,
	host component.Host) {

	for {
		// Blocks until non-empty result or context is cancelled (returns nil in that case)
		envelopes := stream()
		if envelopes == nil {
			// If context has not been cancelled, then nil means the shutdown was due to an error within stream
			if ctx.Err() == nil {
				host.ReportFatalError(errors.New("RLP gateway streamer shut down due to an error"))
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
			err := cfr.nextConsumer.ConsumeMetrics(ctx, metrics)
			cfr.obsrecv.EndMetricsOp(obsCtx, dataFormat, metrics.DataPointCount(), err)
		}
	}
}

func createLibraryMetricsSlice(metrics pmetric.Metrics) pmetric.MetricSlice {
	resourceMetrics := metrics.ResourceMetrics()
	resourceMetric := resourceMetrics.AppendEmpty()
	resourceMetric.Resource().Attributes()
	libraryMetricsSlice := resourceMetric.ScopeMetrics()
	libraryMetrics := libraryMetricsSlice.AppendEmpty()
	libraryMetrics.Scope().SetName(instrumentationLibName)
	return libraryMetrics.Metrics()
}
