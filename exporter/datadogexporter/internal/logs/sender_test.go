// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap/zaptest"
)

func TestSubmitLogs(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name        string
		payload     []datadogV2.HTTPLogItem
		testFn      func(jsonLogs testutil.JSONLogs, call int)
		numRequests int
	}{
		{
			name: "same-tags",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag1:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 1",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag1:true",
					"hostname": "hostname",
					"message":  "log 1",
					"service":  "server",
				},
			}, {
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag1:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 2",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag1:true",
					"hostname": "hostname",
					"message":  "log 2",
					"service":  "server",
				},
			}},
			testFn: func(jsonLogs testutil.JSONLogs, call int) {
				switch call {
				case 0:
					assert.True(t, jsonLogs.HasDDTag("tag1:true"))
					assert.Len(t, jsonLogs, 2)
				default:
					t.Fail()
				}
			},
			numRequests: 1,
		},
		{
			name: "different-tags",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag1:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 1",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag1:true",
					"hostname": "hostname",
					"message":  "log 1",
					"service":  "server",
				},
			}, {
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag2:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 2",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag2:true",
					"hostname": "hostname",
					"message":  "log 2",
					"service":  "server",
				},
			}},
			testFn: func(jsonLogs testutil.JSONLogs, call int) {
				switch call {
				case 0:
					assert.True(t, jsonLogs.HasDDTag("tag1:true"))
					assert.Len(t, jsonLogs, 1)
				case 1:
					assert.True(t, jsonLogs.HasDDTag("tag2:true"))
					assert.Len(t, jsonLogs, 1)
				default:
					t.Fail()
				}
			},
			numRequests: 2,
		},
		{
			name: "two-batches",
			payload: []datadogV2.HTTPLogItem{{
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag1:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 1",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag1:true",
					"hostname": "hostname",
					"message":  "log 1",
					"service":  "server",
				},
			}, {
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag1:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 2",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag1:true",
					"hostname": "hostname",
					"message":  "log 2",
					"service":  "server",
				},
			}, {
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag2:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 3",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag2:true",
					"hostname": "hostname",
					"message":  "log 3",
					"service":  "server",
				},
			}, {
				Ddsource: datadog.PtrString("golang"),
				Ddtags:   datadog.PtrString("tag2:true"),
				Hostname: datadog.PtrString("hostname"),
				Message:  "log 4",
				Service:  datadog.PtrString("server"),
				UnparsedObject: map[string]any{
					"ddsource": "golang",
					"ddtags":   "tag2:true",
					"hostname": "hostname",
					"message":  "log 4",
					"service":  "server",
				},
			}},
			testFn: func(jsonLogs testutil.JSONLogs, call int) {
				switch call {
				case 0:
					assert.True(t, jsonLogs.HasDDTag("tag1:true"))
					assert.Len(t, jsonLogs, 2)
				case 1:
					assert.True(t, jsonLogs.HasDDTag("tag2:true"))
					assert.Len(t, jsonLogs, 2)
				default:
					t.Fail()
				}
			},
			numRequests: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := testutil.DatadogLogServerMock(func() (string, http.HandlerFunc) {
				return "/api/v2/logs", func(writer http.ResponseWriter, request *http.Request) {
					jsonLogs := testutil.MockLogsEndpoint(writer, request)
					tt.testFn(jsonLogs, calls)
					calls++
				}
			})
			defer server.Close()
			s := NewSender(server.URL, logger, confighttp.ClientConfig{Timeout: time.Second * 10, TLSSetting: configtls.ClientConfig{InsecureSkipVerify: true}}, true, "")
			require.NoError(t, s.SubmitLogs(context.Background(), tt.payload))
			assert.Equal(t, calls, tt.numRequests)
		})
	}
}
