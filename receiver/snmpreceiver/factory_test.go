// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.Equal(t, metadata.Type, factory.Type())
			},
		},
		{
			desc: "creates a new factory with valid default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()

				var expectedCfg component.Config = &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: defaultCollectionInterval,
						Timeout:            defaultTimeout,
					},
					Endpoint:      defaultEndpoint,
					Version:       defaultVersion,
					Community:     defaultCommunity,
					SecurityLevel: "no_auth_no_priv",
					AuthType:      "MD5",
					PrivacyType:   "DES",
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Unit:  "1",
						Gauge: &GaugeMetric{ValueType: "int"},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotSNMP)
			},
		},
		{
			desc: "CreateMetrics adds missing scheme to endpoint",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Endpoint = "localhost:161"
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Unit:  "1",
						Gauge: &GaugeMetric{ValueType: "int"},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "udp://localhost:161", snmpCfg.Endpoint)
			},
		},
		{
			desc: "CreateMetrics adds missing port to endpoint",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Endpoint = "udp://localhost"
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Unit:  "1",
						Gauge: &GaugeMetric{ValueType: "int"},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "udp://localhost:161", snmpCfg.Endpoint)
			},
		},
		{
			desc: "CreateMetrics adds missing port to endpoint with trailing colon",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Endpoint = "udp://localhost:"
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Unit:  "1",
						Gauge: &GaugeMetric{ValueType: "int"},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "udp://localhost:161", snmpCfg.Endpoint)
			},
		},
		{
			desc: "CreateMetrics adds missing metric gauge value type as double",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Gauge: &GaugeMetric{},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "double", snmpCfg.Metrics["m1"].Gauge.ValueType)
			},
		},
		{
			desc: "CreateMetrics adds missing metric sum value type as double",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Sum: &SumMetric{},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "double", snmpCfg.Metrics["m1"].Sum.ValueType)
			},
		},
		{
			desc: "CreateMetrics adds missing metric sum aggregation as cumulative",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Sum: &SumMetric{},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "cumulative", snmpCfg.Metrics["m1"].Sum.Aggregation)
			},
		},
		{
			desc: "CreateMetrics adds missing metric unit as 1",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				snmpCfg := cfg.(*Config)
				snmpCfg.Metrics = map[string]*MetricConfig{
					"m1": {
						Gauge: &GaugeMetric{ValueType: "int"},
						ScalarOIDs: []ScalarOID{{
							OID: ".1",
						}},
					},
				}
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.Equal(t, "1", snmpCfg.Metrics["m1"].Unit)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
