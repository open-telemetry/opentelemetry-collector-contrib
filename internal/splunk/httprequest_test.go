// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConsumeMetrics(t *testing.T) {
	tests := []struct {
		name             string
		httpResponseCode int
		retryAfter       int
		wantErr          bool
		wantPermanentErr bool
		wantThrottleErr  bool
		wantRetryableErr bool
	}{
		{
			name:             "response_forbidden",
			httpResponseCode: http.StatusForbidden,
			wantErr:          true,
		},
		{
			name:             "response_bad_request",
			httpResponseCode: http.StatusBadRequest,
			wantPermanentErr: true,
		},
		{
			name:             "response_throttle",
			httpResponseCode: http.StatusTooManyRequests,
			wantThrottleErr:  true,
		},
		{
			name:             "response_throttle_with_header",
			retryAfter:       123,
			httpResponseCode: http.StatusServiceUnavailable,
			wantThrottleErr:  true,
		},
		{
			name:             "large_batch",
			httpResponseCode: http.StatusAccepted,
		},
		// 4xx that retrying can't fix: permanent.
		{name: "not_found", httpResponseCode: http.StatusNotFound, wantPermanentErr: true},
		{name: "method_not_allowed", httpResponseCode: http.StatusMethodNotAllowed, wantPermanentErr: true},
		{name: "conflict", httpResponseCode: http.StatusConflict, wantPermanentErr: true},
		{name: "gone", httpResponseCode: http.StatusGone, wantPermanentErr: true},
		{name: "request_entity_too_large", httpResponseCode: http.StatusRequestEntityTooLarge, wantPermanentErr: true},
		{name: "request_uri_too_long", httpResponseCode: http.StatusRequestURITooLong, wantPermanentErr: true},
		{name: "unsupported_media_type", httpResponseCode: http.StatusUnsupportedMediaType, wantPermanentErr: true},
		{name: "unprocessable_entity", httpResponseCode: http.StatusUnprocessableEntity, wantPermanentErr: true},
		// Transient failures must stay retryable.
		{name: "request_timeout", httpResponseCode: http.StatusRequestTimeout, wantRetryableErr: true},
		{name: "internal_server_error", httpResponseCode: http.StatusInternalServerError, wantRetryableErr: true},
		{name: "bad_gateway", httpResponseCode: http.StatusBadGateway, wantRetryableErr: true},
		{name: "gateway_timeout", httpResponseCode: http.StatusGatewayTimeout, wantRetryableErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Request: &http.Request{
					URL: &url.URL{Scheme: "http", Host: "splunk.com", Path: "/endpoint"},
				},
				StatusCode: tt.httpResponseCode,
			}
			if tt.retryAfter != 0 {
				resp.Header = map[string][]string{
					HeaderRetryAfter: {strconv.Itoa(tt.retryAfter)},
				}
			}

			err := HandleHTTPCode(resp)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			if tt.wantPermanentErr {
				assert.Error(t, err)
				assert.True(t, consumererror.IsPermanent(err))
				return
			}

			if tt.wantThrottleErr {
				expected := fmt.Errorf("HTTP \"/endpoint\" %d %q", tt.httpResponseCode, http.StatusText(tt.httpResponseCode))
				expected = exporterhelper.NewThrottleRetry(expected, time.Duration(tt.retryAfter)*time.Second)
				assert.Equal(t, expected, err)
				return
			}

			if tt.wantRetryableErr {
				assert.Error(t, err)
				assert.False(t, consumererror.IsPermanent(err), "expected a retryable (non-permanent) error")
				return
			}

			assert.NoError(t, err)
		})
	}
}
