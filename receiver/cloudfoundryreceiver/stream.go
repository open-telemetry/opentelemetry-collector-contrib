// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

type envelopeStreamFactory struct {
	rlpGatewayClient *loggregator.RLPGatewayClient
}

func newEnvelopeStreamFactory(
	ctx context.Context,
	settings component.TelemetrySettings,
	authTokenProvider *uaaTokenProvider,
	httpConfig confighttp.ClientConfig,
	host component.Host,
) (*envelopeStreamFactory, error) {
	httpClient, err := httpConfig.ToClient(ctx, host, settings)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client for Cloud Foundry RLP Gateway: %w", err)
	}

	gatewayClient := loggregator.NewRLPGatewayClient(httpConfig.Endpoint,
		loggregator.WithRLPGatewayClientLogger(zap.NewStdLog(settings.Logger)),
		loggregator.WithRLPGatewayHTTPClient(&authorizationProvider{
			logger:            settings.Logger,
			authTokenProvider: authTokenProvider,
			client:            httpClient,
		}),
	)

	return &envelopeStreamFactory{gatewayClient}, nil
}

func (rgc *envelopeStreamFactory) CreateMetricsStream(ctx context.Context, baseShardID string) loggregator.EnvelopeStream {
	newShardID := baseShardID + "_metrics"
	selectors := []*loggregator_v2.Selector{
		{
			Message: &loggregator_v2.Selector_Counter{
				Counter: &loggregator_v2.CounterSelector{},
			},
		},
		{
			Message: &loggregator_v2.Selector_Gauge{
				Gauge: &loggregator_v2.GaugeSelector{},
			},
		},
	}
	stream := rgc.rlpGatewayClient.Stream(ctx, &loggregator_v2.EgressBatchRequest{
		ShardId:   newShardID,
		Selectors: selectors,
	})
	return stream
}

func (rgc *envelopeStreamFactory) CreateLogsStream(ctx context.Context, baseShardID string) loggregator.EnvelopeStream {
	newShardID := baseShardID + "_logs"
	selectors := []*loggregator_v2.Selector{
		{
			Message: &loggregator_v2.Selector_Log{
				Log: &loggregator_v2.LogSelector{},
			},
		},
	}
	stream := rgc.rlpGatewayClient.Stream(ctx, &loggregator_v2.EgressBatchRequest{
		ShardId:   newShardID,
		Selectors: selectors,
	})
	return stream
}

type authorizationProvider struct {
	logger            *zap.Logger
	authTokenProvider *uaaTokenProvider
	client            *http.Client
}

func (ap *authorizationProvider) Do(request *http.Request) (*http.Response, error) {
	token, err := ap.authTokenProvider.ProvideToken()
	if err != nil {
		ap.logger.Error("fetching authentication token", zap.Error(err))
		return nil, errors.New("obtaining authentication token for the request")
	}
	request.Header.Set("Authorization", token)

	return ap.client.Do(request)
}
