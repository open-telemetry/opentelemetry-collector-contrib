// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

func TestConsumeMetrics(t *testing.T) {
	tests := []struct {
		name             string
		httpResponseCode int
		retryAfter       int
		respBody         string
		wantErr          bool
		wantPermanentErr bool
		wantThrottleErr  bool
		wantErrMessage   bool
		noErrMessage     bool
		responseOk       bool
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
		{
			name:             "response_disabled_token",
			httpResponseCode: http.StatusForbidden,
			respBody:         "{\"text\":\"Token disabled\",\"code\":1}",
			wantErrMessage:   true,
		},
		{
			name:             "response_no_text",
			httpResponseCode: http.StatusForbidden,
			respBody:         "{\"code\":1}",
			noErrMessage:     true,
		},
		{
			name:             "response_ok",
			httpResponseCode: http.StatusOK,
			respBody:         "{\"text\":\"ok\"}",
			responseOk:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.httpResponseCode,
			}
			if tt.retryAfter != 0 {
				resp.Header = map[string][]string{
					HeaderRetryAfter: {strconv.Itoa(tt.retryAfter)},
				}
			}

			if tt.respBody != "" {
				resp.Body = io.NopCloser(strings.NewReader(tt.respBody))
				resp.ContentLength = int64(len(tt.respBody))
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
				expected := fmt.Errorf("HTTP %d %q", tt.httpResponseCode, http.StatusText(tt.httpResponseCode))
				expected = exporterhelper.NewThrottleRetry(expected, time.Duration(tt.retryAfter)*time.Second)
				assert.EqualValues(t, expected, err)
				return
			}

			if tt.wantErrMessage {
				assert.Error(t, err)
				expected := fmt.Errorf("HTTP %d %q", tt.httpResponseCode, http.StatusText(tt.httpResponseCode))
				expected = multierr.Append(expected, fmt.Errorf("reason: %v", "Token disabled"))
				assert.EqualValues(t, expected, err)
				return
			}

			if tt.noErrMessage {
				assert.Error(t, err)
				expected := fmt.Errorf("HTTP %d %q", tt.httpResponseCode, http.StatusText(tt.httpResponseCode))
				expected = multierr.Append(expected, fmt.Errorf("no error message from Splunk found"))
				assert.EqualValues(t, expected, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
