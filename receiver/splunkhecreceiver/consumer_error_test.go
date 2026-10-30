// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

func TestConsumerErrorHTTPResponse(t *testing.T) {
	for _, signal := range []struct {
		name      string
		body      string
		handle    func(*splunkReceiver, http.ResponseWriter, *http.Request)
		configure func(*splunkReceiver, error)
		wrap      func(error) error
	}{
		{
			name:   "event logs",
			body:   `{"event":"test event","index":"test-index"}`,
			handle: (*splunkReceiver).handleReq,
			configure: func(r *splunkReceiver, err error) {
				r.logsConsumer = consumertest.NewErr(err)
			},
			wrap: func(err error) error { return consumererror.NewLogs(err, plog.NewLogs()) },
		},
		{
			name:   "raw logs",
			body:   "test event",
			handle: (*splunkReceiver).handleRawReq,
			configure: func(r *splunkReceiver, err error) {
				r.logsConsumer = consumertest.NewErr(err)
			},
			wrap: func(err error) error { return consumererror.NewLogs(err, plog.NewLogs()) },
		},
		{
			name:   "metrics",
			body:   `{"event":"metric","fields":{"metric_name:test":1}}`,
			handle: (*splunkReceiver).handleReq,
			configure: func(r *splunkReceiver, err error) {
				r.metricsConsumer = consumertest.NewErr(err)
			},
			wrap: func(err error) error { return consumererror.NewMetrics(err, pmetric.NewMetrics()) },
		},
	} {
		t.Run(signal.name, func(t *testing.T) {
			cause := errors.New("downstream consumer failed")
			for _, tt := range []struct {
				name   string
				err    error
				status int
			}{
				{name: "success", status: http.StatusOK},
				{name: "transient", err: cause, status: http.StatusInternalServerError},
				{name: "permanent", err: consumererror.NewPermanent(cause), status: http.StatusBadRequest},
				{
					name:   "wrapped transient",
					err:    fmt.Errorf("export failed: %w", signal.wrap(cause)),
					status: http.StatusInternalServerError,
				},
				{
					name:   "wrapped permanent",
					err:    fmt.Errorf("export failed: %w", signal.wrap(consumererror.NewPermanent(cause))),
					status: http.StatusBadRequest,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					cfg := createDefaultConfig().(*Config)
					cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
					r, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *cfg)
					require.NoError(t, err)
					signal.configure(r, tt.err)

					req := httptest.NewRequest(http.MethodPost, "http://localhost", strings.NewReader(signal.body))
					resp := httptest.NewRecorder()
					signal.handle(r, resp, req)

					assert.Equal(t, tt.status, resp.Code)
					if tt.err == nil {
						assert.JSONEq(t, responseOK, resp.Body.String())
					} else {
						assert.JSONEq(t, responseErrInternalServerError, resp.Body.String())
					}
				})
			}
		})
	}
}
