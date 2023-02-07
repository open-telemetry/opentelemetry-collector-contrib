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

package logs

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
)

// validateTags verifies that all log items in the same batch have the same ddtags as the given tag
func validateTags(t *testing.T, jsonLogs testutil.JsonLogs, tag string) {
	for _, log := range jsonLogs {
		assert.Equal(t, log["ddtags"], tag)
	}
}

func TestSubmitLogs(t *testing.T) {
	logger := zaptest.NewLogger(t)
	var counter int

	tests := []struct {
		name    string
		payload []datadogV2.HTTPLogItem
		testFn  func(jsonLogs testutil.JsonLogs)
	}{
		{
			name: "batches same tags",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag1:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}, {
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag1:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}},
			testFn: func(jsonLogs testutil.JsonLogs) {
				switch counter {
				case 0:
					validateTags(t, jsonLogs, "tag1:true")
					assert.Len(t, jsonLogs, 2)
				default:
					t.Fail()
				}
				counter++
			},
		},
		{
			name: "does not batch same tags",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag1:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}, {
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag2:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}},
			testFn: func(jsonLogs testutil.JsonLogs) {
				switch counter {
				case 0:
					validateTags(t, jsonLogs, "tag1:true")
					assert.Len(t, jsonLogs, 1)
				case 1:
					validateTags(t, jsonLogs, "tag2:true")
					assert.Len(t, jsonLogs, 1)
				default:
					t.Fail()
				}
				counter++
			},
		},
		{
			name: "does two batches",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag1:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}, {
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag1:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}, {
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag2:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}, {
				Ddsource:             nil,
				Ddtags:               datadog.PtrString("tag2:true"),
				Hostname:             nil,
				Message:              "",
				Service:              nil,
				UnparsedObject:       nil,
				AdditionalProperties: nil,
			}},
			testFn: func(jsonLogs testutil.JsonLogs) {
				switch counter {
				case 0:
					validateTags(t, jsonLogs, "tag1:true")
					assert.Len(t, jsonLogs, 2)
				case 1:
					validateTags(t, jsonLogs, "tag2:true")
					assert.Len(t, jsonLogs, 2)
				default:
					t.Fail()
				}
				counter++
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.DatadogLogServerMock(func() (string, http.HandlerFunc) {
				return "/api/v2/logs", func(writer http.ResponseWriter, request *http.Request) {
					jsonLogs := testutil.MockLogsEndpoint(writer, request)
					tt.testFn(jsonLogs)
				}
			})
			defer server.Close()
			counter = 0
			s := NewSender(server.URL, logger, exporterhelper.TimeoutSettings{Timeout: time.Second * 10}, true, true, "")
			_ = s.SubmitLogs(context.Background(), tt.payload)
		})
	}
}
