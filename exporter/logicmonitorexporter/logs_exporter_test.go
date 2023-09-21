// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	lmsdklogs "github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/testutil"
)

func Test_NewLogsExporter(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			config *Config
			logger *zap.Logger
		}
	}{
		{
			name: "should create LogExporter",
			args: struct {
				config *Config
				logger *zap.Logger
			}{
				config: &Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "http://example.logicmonitor.com/rest",
					},
					APIToken: APIToken{AccessID: "testid", AccessKey: "testkey"},
				},
				logger: zaptest.NewLogger(t),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := exportertest.NewNopCreateSettings()
			exp := newLogsExporter(context.Background(), tt.args.config, set)
			assert.NotNil(t, exp)
		})
	}
}

func TestPushLogData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := lmsdklogs.LMLogIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		w.WriteHeader(http.StatusAccepted)
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))
	defer ts.Close()

	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: ts.URL,
		},
		APIToken: APIToken{AccessID: "testid", AccessKey: "testkey"},
	}

	tests := []struct {
		name   string
		fields struct {
			config *Config
			logger *zap.Logger
		}
		args struct {
			ctx context.Context
			lg  plog.Logs
		}
	}{
		{
			name: "Send Log data",
			fields: struct {
				config *Config
				logger *zap.Logger
			}{
				logger: zaptest.NewLogger(t),
				config: cfg,
			},
			args: struct {
				ctx context.Context
				lg  plog.Logs
			}{
				ctx: context.Background(),
				lg:  testutil.CreateLogData(1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set := exportertest.NewNopCreateSettings()
			exp := newLogsExporter(test.args.ctx, test.fields.config, set)

			require.NoError(t, exp.start(test.args.ctx, componenttest.NewNopHost()))
			err := exp.PushLogData(test.args.ctx, test.args.lg)
			assert.NoError(t, err)
		})
	}
}
