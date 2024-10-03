// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	defaultMaxIdleConns := http.DefaultTransport.(*http.Transport).MaxIdleConns
	defaultMaxIdleConnsPerHost := http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost
	defaultMaxConnsPerHost := http.DefaultTransport.(*http.Transport).MaxConnsPerHost
	defaultIdleConnTimeout := http.DefaultTransport.(*http.Transport).IdleConnTimeout

	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.EqualValues(t, metadata.Type, factory.Type())
			},
		},
		{
			desc: "creates a new factory with valid default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()

				var expectedCfg component.Config = &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: defaultCollectionInterval,
						InitialDelay:       time.Second,
					},
					ClientConfig: confighttp.ClientConfig{
						Endpoint:            defaultEndpoint,
						MaxIdleConns:        &defaultMaxIdleConns,
						MaxIdleConnsPerHost: &defaultMaxIdleConnsPerHost,
						MaxConnsPerHost:     &defaultMaxConnsPerHost,
						IdleConnTimeout:     &defaultIdleConnTimeout,
						Headers:             map[string]configopaque.String{},
					},
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetricsReceiver returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetricsReceiver(
					context.Background(),
					receivertest.NewNopSettings(),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotSpark)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
