// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nsxreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type receiverOps func(t *testing.T, rcvr *nsxReceiver)

func TestStart(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	cases := []struct {
		desc          string
		config        *Config
		ops           []receiverOps
		expectedError error
	}{
		{
			desc:   "default",
			config: defaultConfig,
		},
		{
			desc: "metrics config",
			config: &Config{
				MetricsConfig: &MetricsConfig{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(defaultConfig.ID().Type()),
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "https://localhost/",
					},
				},
			},
			ops: []receiverOps{metricFactoryCreation},
		},
		{
			desc: "metrics config failure via control character",
			config: &Config{
				MetricsConfig: &MetricsConfig{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(defaultConfig.ID().Type()),
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: string([]byte{0x7f}),
					},
				},
			},
			ops: []receiverOps{metricFactoryCreation},
		},
		{
			desc: "metrics config failure via control character, should not fail ",
			config: &Config{
				MetricsConfig: &MetricsConfig{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(defaultConfig.ID().Type()),
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: string([]byte{0x7f}),
					},
				},
			},
			ops: []receiverOps{metricFactoryCreation},
		},
		{
			desc: "syslogreceiver not packaged",
			config: &Config{
				MetricsConfig: defaultConfig.MetricsConfig,
				LoggingConfig: defaultConfig.LoggingConfig,
			},
			ops:           []receiverOps{loggingFactoryCreation},
			expectedError: errors.New("unable to wrap the syslog receiver"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			rcvr := &nsxReceiver{
				config: tc.config,
			}

			for _, op := range tc.ops {
				op(t, rcvr)
			}

			err := rcvr.Start(context.Background(), componenttest.NewNopHost())
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

var metricFactoryCreation = func(t *testing.T, rcvr *nsxReceiver) {
	factory := &nsxReceiverFactory{
		receivers: map[*Config]*nsxReceiver{},
	}
	sink := &consumertest.MetricsSink{}
	mr, err := factory.createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), rcvr.config, sink)
	require.NoError(t, err)
	rcvr.scraper = mr
}

var loggingFactoryCreation = func(t *testing.T, rcvr *nsxReceiver) {
	factory := &nsxReceiverFactory{
		receivers: map[*Config]*nsxReceiver{},
	}
	sink := &consumertest.LogsSink{}
	lr, err := factory.createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), rcvr.config, sink)
	require.NoError(t, err)
	rcvr.logsReceiver = lr
}

func TestShutdown(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	cases := []struct {
		desc          string
		config        *Config
		ops           []receiverOps
		expectedError error
	}{
		{
			desc:   "default",
			config: defaultConfig,
		},
		{
			desc:   "with metrics receiver",
			config: defaultConfig,
			ops:    []receiverOps{metricFactoryCreation},
		},
		{
			desc:   "with logging receiver",
			config: defaultConfig,
			ops:    []receiverOps{loggingFactoryCreation},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			rcvr := &nsxReceiver{
				config: tc.config,
			}
			for _, op := range tc.ops {
				op(t, rcvr)
			}
			_ = rcvr.Start(context.Background(), componenttest.NewNopHost())
			err := rcvr.Shutdown(context.Background())
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
