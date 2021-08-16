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

package cloudfoundryreceiver

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.uber.org/zap"
)

type EnvelopeStreamProvider struct {
	rlpGatewayClient *loggregator.RLPGatewayClient
}

func newEnvelopeStreamProvider(
	logger *zap.Logger,
	authTokenProvider *UAATokenProvider,
	rlpGatewayURL string,
	tlsSkipVerify bool,
	httpTimeout time.Duration) EnvelopeStreamProvider {

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		},
		DialContext: (&net.Dialer{
			Timeout: httpTimeout,
		}).DialContext,
	}

	gatewayClient := loggregator.NewRLPGatewayClient(rlpGatewayURL,
		loggregator.WithRLPGatewayClientLogger(zap.NewStdLog(logger)),
		loggregator.WithRLPGatewayHTTPClient(&authorizationProvider{
			authTokenProvider: authTokenProvider,
			client: &http.Client{
				Transport: &transport,
			},
		}),
	)

	return EnvelopeStreamProvider{gatewayClient}
}

func (rgc *EnvelopeStreamProvider) CreateStream(
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
		ap.logger.Error("failed to fetch authentication token", zap.Error(err))
	}

	return ap.client.Do(request)
}
