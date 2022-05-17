// Copyright 2022, OpenTelemetry Authors
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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"errors"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

func TestNewAerospikeReceiver_BadEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		errMsg   string
	}{
		{
			name:     "no port",
			endpoint: "localhost",
			errMsg:   "missing port in address",
		},
		{
			name:     "no address",
			endpoint: "",
			errMsg:   "missing port in address",
		},
	}

	cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Endpoint: tc.endpoint}
			receiver, err := newAerospikeReceiver(component.ReceiverCreateSettings{}, cfg, cs)
			require.ErrorContains(t, err, tc.errMsg)
			require.Nil(t, receiver)
		})
	}
}

func TestScrapeNode(t *testing.T) {
	testCases := []struct {
		name                 string
		setupClient          func() *mocks.Aerospike
		setupExpectedMetrics func(t *testing.T) pdata.Metrics
		expectedErr          string
	}{
		{
			name: "error response",
			setupClient: func() *mocks.Aerospike {
				client := &mocks.Aerospike{}
				client.On("Info").Return(nil, as.ErrNetTimeout)
				return client
			},
			setupExpectedMetrics: func(t *testing.T) pdata.Metrics {
				return pdata.NewMetrics()
			},
			expectedErr: as.ErrNetTimeout.Error(),
		},
		{
			name: "empty response",
			setupClient: func() *mocks.Aerospike {
				client := &mocks.Aerospike{}
				client.On("Info").Return(&model.NodeInfo{}, nil)
				return client
			},
			setupExpectedMetrics: func(t *testing.T) pdata.Metrics {
				return pdata.NewMetrics()
			},
		},
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.setupClient()
			cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
			require.NoError(t, err)

			receiver, err := newAerospikeReceiver(
				component.ReceiverCreateSettings{
					TelemetrySettings: component.TelemetrySettings{
						Logger: logger,
					},
				},
				&Config{Endpoint: "localhost:3000"},
				cs,
			)
			require.NoError(t, err)
			errs := &scrapererror.ScrapeErrors{}
			receiver.scrapeNode(client, pcommon.NewTimestampFromTime(time.Now().UTC()), errs)

			if tc.expectedErr != "" {
				assert.EqualError(t, errs.Combine(), tc.expectedErr)
			}
			expectedMetrics := tc.setupExpectedMetrics(t)
			client.AssertExpectations(t)
			scrapertest.CompareMetrics(expectedMetrics, receiver.mb.Emit())
		})
	}
}

func TestScrape(t *testing.T) {
	testCases := []struct {
		name            string
		config          *Config
		expectedMetrics pmetric.Metrics
		expectedErr     error
	}{
		{
			name: "bad host",
			config: &Config{
				Endpoint: "local.invalid:3000",
			},
			expectedMetrics: pmetric.NewMetrics(),
			expectedErr:     errors.New("failed to connect"),
		},
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	for _, tc := range testCases {
		cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
		require.NoError(t, err)

		receiver, err := newAerospikeReceiver(component.ReceiverCreateSettings{
			TelemetrySettings: component.TelemetrySettings{
				Logger: logger,
			},
		}, tc.config, cs)
		require.NoError(t, err)

		metrics, err := receiver.scrape(context.Background())
		require.Equal(t, tc.expectedMetrics, metrics)

		if tc.expectedErr != nil {
			require.ErrorContains(t, err, tc.expectedErr.Error())
		}
	}
}
