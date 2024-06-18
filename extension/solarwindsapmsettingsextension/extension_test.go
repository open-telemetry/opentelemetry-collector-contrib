// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/solarwinds/apm-proto/go/collectorpb"
	"github.com/solarwinds/apm-proto/go/collectorpb/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCreateExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default",
			cfg: &Config{
				Endpoint: DefaultEndpoint,
				Interval: DefaultInterval,
			},
		},
		{
			name: "anything",
			cfg: &Config{
				Endpoint: "apm.collector.na-02.cloud.solarwinds.com:443",
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := createAnExtension(tt.cfg, t)
			require.NoError(t, ex.Shutdown(context.TODO()))
		})
	}
}

// create extension
func createAnExtension(c *Config, t *testing.T) extension.Extension {
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	ex, err := newSolarwindsApmSettingsExtension(c, logger)
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
				Endpoint: DefaultEndpoint,
			},
			reply: &collectorpb.SettingsResult{
				Result: collectorpb.ResultCode_OK,
				Settings: []*collectorpb.OboeSetting{
					&collectorpb.OboeSetting{
						Type:      collectorpb.OboeSettingType_DEFAULT_SAMPLE_RATE,
						Flags:     []byte("flags"),
						Timestamp: 0,
						Value:     0,
						Layer:     []byte{},
						Arguments: map[string][]byte{
							"BucketCapacity":               []byte{1, 0, 0, 0, 0, 0, 0, 64},
							"BucketRate":                   []byte{2, 0, 0, 0, 0, 0, 240, 63},
							"TriggerRelaxedBucketCapacity": []byte{3, 0, 0, 0, 0, 0, 52, 64},
							"TriggerRelaxedBucketRate":     []byte{4, 0, 0, 0, 0, 0, 240, 63},
							"TriggerStrictBucketCapacity":  []byte{0, 0, 0, 0, 0, 0, 24, 64},
							"TriggerStrictBucketRate":      []byte{154, 153, 153, 153, 153, 153, 185, 63},
							"MetricsFlushInterval":         []byte{60, 0, 0, 0},
							"MaxTransactions":              []byte{1, 0, 0, 0},
							"MaxCustomMetrics":             []byte{2, 0, 0, 0},
							"EventsFlushInterval":          []byte{3, 0, 0, 0},
							"ProfilingInterval":            []byte{4, 0, 0, 0},
							"SignatureKey":                 []byte("key"),
						},
						Ttl: 12,
					},
				},
			},
			filename: "testdata/refresh_ok.json",
			expectedLogMessages: []string{
				"time to refresh",
				"testdata/refresh_ok.json is refreshed",
				"[{\"arguments\":{\"BucketCapacity\":2.0000000000000004,\"BucketRate\":1.0000000000000004,\"EventsFlushInterval\":3,\"MaxCustomMetrics\":2,\"MaxTransactions\":1,\"MetricsFlushInterval\":60,\"ProfilingInterval\":4,\"TriggerRelaxedBucketCapacity\":20.00000000000001,\"TriggerRelaxedBucketRate\":1.0000000000000009,\"TriggerStrictBucketCapacity\":6,\"TriggerStrictBucketRate\":0.1},\"flags\":\"flags\",\"layer\":\"\",\"timestamp\":0,\"ttl\":12,\"type\":0,\"value\":0}]",
			},
			fileExist: true,
		},
		{
			name: "ok with warning",
			cfg: &Config{
				Endpoint: DefaultEndpoint,
			},
			reply: &collectorpb.SettingsResult{
				Result: collectorpb.ResultCode_OK,
				Settings: []*collectorpb.OboeSetting{
					&collectorpb.OboeSetting{
						Type:      collectorpb.OboeSettingType_DEFAULT_SAMPLE_RATE,
						Flags:     []byte("flags"),
						Timestamp: 0,
						Value:     0,
						Layer:     []byte{},
						Arguments: map[string][]byte{
							"BucketCapacity":               []byte{0, 0, 0, 0, 0, 0, 0, 0},
							"BucketRate":                   []byte{2, 0, 0, 0, 0, 0, 0, 0},
							"TriggerRelaxedBucketCapacity": []byte{0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerRelaxedBucketRate":     []byte{0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerStrictBucketCapacity":  []byte{0, 0, 0, 0, 0, 0, 0, 0},
							"TriggerStrictBucketRate":      []byte{0, 0, 0, 0, 0, 0, 0, 0},
							"MetricsFlushInterval":         []byte{0, 0, 0, 0},
							"MaxTransactions":              []byte{0, 0, 0, 0},
							"MaxCustomMetrics":             []byte{0, 0, 0, 0},
							"EventsFlushInterval":          []byte{0, 0, 0, 0},
							"ProfilingInterval":            []byte{0, 0, 0, 0},
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
				"[{\"arguments\":{\"BucketCapacity\":0,\"BucketRate\":1e-323,\"EventsFlushInterval\":0,\"MaxCustomMetrics\":0,\"MaxTransactions\":0,\"MetricsFlushInterval\":0,\"ProfilingInterval\":0,\"TriggerRelaxedBucketCapacity\":0,\"TriggerRelaxedBucketRate\":0,\"TriggerStrictBucketCapacity\":0,\"TriggerStrictBucketRate\":0},\"flags\":\"flags\",\"layer\":\"\",\"timestamp\":0,\"ttl\":10,\"type\":0,\"value\":0}]",
			},
			fileExist: true,
		},
		{
			name: "try later",
			cfg: &Config{
				Endpoint: DefaultEndpoint,
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
				Endpoint: DefaultEndpoint,
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
				Endpoint: DefaultEndpoint,
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
				Endpoint: DefaultEndpoint,
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
			logger := zap.New(observedZapCore)

			settingsExtension := &solarwindsapmSettingsExtension{
				config: tt.cfg,
				logger: logger,
				client: mockTraceCollectorClient,
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
