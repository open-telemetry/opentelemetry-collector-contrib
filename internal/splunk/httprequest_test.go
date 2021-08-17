// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunk

import (
	"fmt"
	"net/http"
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

			assert.NoError(t, err)
		})
	}
}
