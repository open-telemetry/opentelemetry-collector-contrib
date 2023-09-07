// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

type EnvelopeStreamFactory struct {
	rlpGatewayClient *loggregator.RLPGatewayClient
}

func newEnvelopeStreamFactory(
	settings component.TelemetrySettings,
	authTokenProvider *UAATokenProvider,
	httpConfig confighttp.HTTPClientSettings,
	host component.Host) (*EnvelopeStreamFactory, error) {

	httpClient, err := httpConfig.ToClient(host, settings)
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

	return &EnvelopeStreamFactory{gatewayClient}, nil
}

func (rgc *EnvelopeStreamFactory) CreateStream(
	ctx context.Context,
	shardID string) (loggregator.EnvelopeStream, error) {

	if strings.TrimSpace(shardID) == "" {
		return nil, errors.New("shardID cannot be empty")
	}

	stream := rgc.rlpGatewayClient.Stream(ctx, &loggregator_v2.EgressBatchRequest{
		ShardId: shardID,
		Selectors: []*loggregator_v2.Selector{
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
		},
	})

	return stream, nil
}

type authorizationProvider struct {
	logger            *zap.Logger
	authTokenProvider *UAATokenProvider
	client            *http.Client
}

func (ap *authorizationProvider) Do(request *http.Request) (*http.Response, error) {
	token, err := ap.authTokenProvider.ProvideToken()
	if err == nil {
		request.Header.Set("Authorization", token)
	} else {
		ap.logger.Error("fetching authentication token", zap.Error(err))
		return nil, errors.New("obtaining authentication token for the request")
	}

	return ap.client.Do(request)
}
