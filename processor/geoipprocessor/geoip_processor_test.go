// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
)

type providerConfigMock struct {
	ValidateF func() error
}

type providerFactoryMock struct {
	CreateDefaultConfigF func() provider.Config
	CreateGeoIPProviderF func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error)
}

type providerMock struct {
	LocationF func(context.Context, net.IP) (attribute.Set, error)
}

var (
	_ provider.GeoIPProvider        = (*providerMock)(nil)
	_ provider.GeoIPProvider        = (*providerMock)(nil)
	_ provider.GeoIPProviderFactory = (*providerFactoryMock)(nil)
)

func (cm *providerConfigMock) Validate() error {
	return cm.ValidateF()
}

func (fm *providerFactoryMock) CreateDefaultConfig() provider.Config {
	return fm.CreateDefaultConfigF()
}

func (fm *providerFactoryMock) CreateGeoIPProvider(ctx context.Context, settings processor.Settings, cfg provider.Config) (provider.GeoIPProvider, error) {
	return fm.CreateGeoIPProviderF(ctx, settings, cfg)
}

func (pm *providerMock) Location(ctx context.Context, ip net.IP) (attribute.Set, error) {
	return pm.LocationF(ctx, ip)
}

var baseMockProvider = providerMock{
	LocationF: func(context.Context, net.IP) (attribute.Set, error) {
		return attribute.Set{}, nil
	},
}

var baseMockFactory = providerFactoryMock{
	CreateDefaultConfigF: func() provider.Config {
		return &providerConfigMock{ValidateF: func() error { return nil }}
	},
	CreateGeoIPProviderF: func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error) {
		return &baseMockProvider, nil
	},
}

var baseProviderMock = providerMock{
	LocationF: func(context.Context, net.IP) (attribute.Set, error) {
		return attribute.Set{}, nil
	},
}

func TestProcessor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		goldenDir        string
		sourceConfig     SourceConfig
		geoLocationMock  func(context.Context, net.IP) (attribute.Set, error)
		lookupAttributes []attribute.Key
	}{
		{
			name:             "default source.address attribute, not found",
			goldenDir:        "no_source_address",
			sourceConfig:     SourceConfig{From: ResourceSource},
			lookupAttributes: defaultResourceAttributes,
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
		{
			name:             "default source.address attribute",
			goldenDir:        "source_address",
			sourceConfig:     SourceConfig{From: ResourceSource},
			lookupAttributes: defaultResourceAttributes,
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
		{
			name:             "default source.ip attribute with an unspecified IP address should be skipped",
			goldenDir:        "unspecified_address",
			sourceConfig:     SourceConfig{From: ResourceSource},
			lookupAttributes: defaultResourceAttributes,
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
		{
			name:             "custom source attributes",
			goldenDir:        "custom_sources",
			sourceConfig:     SourceConfig{From: ResourceSource},
			lookupAttributes: []attribute.Key{"ip", "host.ip"},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				// only one attribute should be added as we are using a set
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
		{
			name:             "do not add resource attributes with an invalid ip",
			goldenDir:        "invalid_address",
			sourceConfig:     SourceConfig{From: ResourceSource},
			lookupAttributes: defaultResourceAttributes,
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
		{
			name:             "source address located in inner attributes",
			goldenDir:        "attribute_source_address",
			sourceConfig:     SourceConfig{From: AttributeSource},
			lookupAttributes: defaultResourceAttributes,
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseProviderMock.LocationF = tt.geoLocationMock
			processor := newGeoIPProcessor(tt.sourceConfig, tt.lookupAttributes, []provider.GeoIPProvider{&baseProviderMock})

			dir := filepath.Join("testdata", tt.goldenDir)

			// compare metrics
			inputMetrics, err := golden.ReadMetrics(filepath.Join(dir, "input-metrics.yaml"))
			require.NoError(t, err)

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(dir, "output-metrics.yaml"))
			require.NoError(t, err)

			actualMetrics, err := processor.processMetrics(context.Background(), inputMetrics)
			// golden.WriteMetrics(t, filepath.Join(dir, "output-metrics.yaml"), actualMetrics)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))

			// compare metrics
			inputTraces, err := golden.ReadTraces(filepath.Join(dir, "input-traces.yaml"))
			require.NoError(t, err)

			expectedTraces, err := golden.ReadTraces(filepath.Join(dir, "output-traces.yaml"))
			require.NoError(t, err)

			actualTraces, err := processor.processTraces(context.Background(), inputTraces)
			// golden.WriteTraces(t, filepath.Join(dir, "output-traces.yaml"), actualTraces)
			require.NoError(t, err)
			require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces))

			// compare logs
			inputLogs, err := golden.ReadLogs(filepath.Join(dir, "input-logs.yaml"))
			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "output-logs.yaml"))
			require.NoError(t, err)

			actualLogs, err := processor.processLogs(context.Background(), inputLogs)
			// golden.WriteLogs(t, filepath.Join(dir, "output-logs.yaml"), actualLogs)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs))
		})
	}
}
