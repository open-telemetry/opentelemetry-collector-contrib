// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/solarwindscloud/apm-proto/go/collectorpb"
	"github.com/solarwindscloud/apm-proto/go/collectorpb/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Interval: DefaultInterval,
			},
		},
		{
			name: "anything",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.na-02.cloud.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := createAnExtension(t, tt.cfg)
			require.NoError(t, ex.Shutdown(context.TODO()))
		})
	}
}

// newNopSettings returns a new nop settings for extension.Factory Create* functions.
func newNopSettings() extension.Settings {
	return extension.Settings{
		ID:                component.NewIDWithName(component.MustNewType("nop"), uuid.NewString()),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}

// create extension
func createAnExtension(t *testing.T, c *Config) extension.Extension {
	ex, err := newSolarwindsApmSettingsExtension(c, newNopSettings())
	require.NoError(t, err)
	require.NoError(t, ex.Start(context.TODO(), nil))
	return ex
}

func TestRefresh(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		cfg                 *Config
		reply               *collectorpb.SettingsResult
		filename            string
		expectedLogMessages []string
		fileExist           bool
	}{
		{
			name: "ok",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result: collectorpb.ResultCode_OK,
				Settings: []*collectorpb.OboeSetting{
					{
						Type:      collectorpb.OboeSettingType_DEFAULT_SAMPLE_RATE,
						Flags:     []byte("flag1,flag2,flag3"),
						Timestamp: 123,
						Value:     456,
						Layer:     []byte("layer1"),
						Arguments: map[string][]byte{
							"BucketCapacity":               {1, 0, 0, 0, 0, 0, 0, 64},
							"BucketRate":                   {2, 0, 0, 0, 0, 0, 240, 63},
							"TriggerRelaxedBucketCapacity": {3, 0, 0, 0, 0, 0, 52, 64},
							"TriggerRelaxedBucketRate":     {4, 0, 0, 0, 0, 0, 240, 63},
							"TriggerStrictBucketCapacity":  {0, 0, 0, 0, 0, 0, 24, 64},
							"TriggerStrictBucketRate":      {154, 153, 153, 153, 153, 153, 185, 63},
							"MetricsFlushInterval":         {60, 0, 0, 0},
							"MaxTransactions":              {1, 0, 0, 0},
							"MaxCustomMetrics":             {2, 0, 0, 0},
							"EventsFlushInterval":          {3, 0, 0, 0},
							"ProfilingInterval":            {4, 0, 0, 0},
							"SignatureKey":                 []byte("key"),
						},
						Ttl: 789,
					},
				},
			},
			filename: "testdata/refresh_ok.json",
			expectedLogMessages: []string{
				"time to refresh",
				"testdata/refresh_ok.json is refreshed",
				"[{\"arguments\":{\"BucketCapacity\":2.0000000000000004,\"BucketRate\":1.0000000000000004,\"EventsFlushInterval\":3,\"MaxCustomMetrics\":2,\"MaxTransactions\":1,\"MetricsFlushInterval\":60,\"ProfilingInterval\":4,\"TriggerRelaxedBucketCapacity\":20.00000000000001,\"TriggerRelaxedBucketRate\":1.0000000000000009,\"TriggerStrictBucketCapacity\":6,\"TriggerStrictBucketRate\":0.1},\"flags\":\"flag1,flag2,flag3\",\"timestamp\":123,\"ttl\":789,\"value\":456}]",
			},
			fileExist: true,
		},
		{
			name: "ok with warning",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result: collectorpb.ResultCode_OK,
				Settings: []*collectorpb.OboeSetting{
					{
						Type:      collectorpb.OboeSettingType_DEFAULT_SAMPLE_RATE,
						Flags:     []byte("flags"),
						Timestamp: 0,
						Value:     0,
						Layer:     []byte{},
						Arguments: map[string][]byte{
							"BucketCapacity":               {0, 0, 0, 0, 0, 0, 0, 0},
							"BucketRate":                   {2, 0, 0, 0, 0, 0, 0, 0},
							"TriggerRelaxedBucketCapacity": {0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerRelaxedBucketRate":     {0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerStrictBucketCapacity":  {0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerStrictBucketRate":      {0, 0, 0, 0, 0, 0, 0, 0},
							"MetricsFlushInterval":         {0, 0, 0, 0},
							"MaxTransactions":              {0, 0, 0, 0},
							"MaxCustomMetrics":             {0, 0, 0, 0},
							"EventsFlushInterval":          {0, 0, 0, 0},
							"ProfilingInterval":            {0, 0, 0, 0},
							"SignatureKey":                 []byte(""),
						},
						Ttl: 10,
					},
				},
				Warning: "warning",
			},
			filename: "testdata/refresh_warning.json",
			expectedLogMessages: []string{
				"time to refresh",
				"GetSettings succeed",
				"testdata/refresh_warning.json is refreshed (soft disabled)",
				"[{\"arguments\":{\"BucketCapacity\":0,\"BucketRate\":1e-323,\"EventsFlushInterval\":0,\"MaxCustomMetrics\":0,\"MaxTransactions\":0,\"MetricsFlushInterval\":0,\"ProfilingInterval\":0,\"TriggerRelaxedBucketCapacity\":0,\"TriggerRelaxedBucketRate\":0,\"TriggerStrictBucketCapacity\":0,\"TriggerStrictBucketRate\":0},\"flags\":\"flags\",\"timestamp\":0,\"ttl\":10,\"value\":0}]",
			},
			fileExist: true,
		},
		{
			name: "try later",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result:  collectorpb.ResultCode_TRY_LATER,
				Warning: "warning",
			},
			filename: "testdata/refresh_later.json",
			expectedLogMessages: []string{
				"time to refresh",
				"GetSettings failed",
			},
			fileExist: false,
		},
		{
			name: "invalid api key",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result:  collectorpb.ResultCode_INVALID_API_KEY,
				Warning: "warning",
			},
			filename: "testdata/refresh_invalid_api_key.json",
			expectedLogMessages: []string{
				"time to refresh",
				"GetSettings failed",
			},
			fileExist: false,
		},
		{
			name: "limit exceeded",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result:  collectorpb.ResultCode_LIMIT_EXCEEDED,
				Warning: "warning",
			},
			filename: "testdata/refresh_limit.json",
			expectedLogMessages: []string{
				"time to refresh",
				"GetSettings failed",
			},
			fileExist: false,
		},
		{
			name: "redirect",
			cfg: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
			},
			reply: &collectorpb.SettingsResult{
				Result:  collectorpb.ResultCode_REDIRECT,
				Warning: "warning",
			},
			filename: "testdata/refresh_redirect.json",
			expectedLogMessages: []string{
				"time to refresh",
				"GetSettings failed",
			},
			fileExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTraceCollectorClient := &mocks.TraceCollectorClient{}
			mockTraceCollectorClient.On("GetSettings", mock.Anything, mock.Anything).Return(tt.reply, nil)
			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			settings := newNopSettings()
			settings.TelemetrySettings.Logger = zap.New(observedZapCore)
			settingsExtension := &solarwindsapmSettingsExtension{
				config:            tt.cfg,
				telemetrySettings: settings.TelemetrySettings,
				client:            mockTraceCollectorClient,
			}
			refresh(settingsExtension, tt.filename)
			require.Equal(t, len(tt.expectedLogMessages), observedLogs.Len())
			for index, observedLog := range observedLogs.All() {
				require.Equal(t, tt.expectedLogMessages[index], observedLog.Message)
			}
			_, err := os.Stat(tt.filename)
			if tt.fileExist {
				require.NoError(t, err)
				require.NoError(t, os.Remove(tt.filename))
			} else {
				require.Error(t, err)
			}
		})
	}
}
