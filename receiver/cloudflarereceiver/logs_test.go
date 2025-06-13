// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/metadata"
)

func TestPayloadToLogRecord(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name         string
		payload      string
		expectedLogs func(*testing.T, string) plog.Logs
		expectedErr  string
	}{
		{
			name: "limited records",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": 69045, "EdgeResponseStatus": 200, "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87" }
{ "ClientIP": "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": 69045, "EdgeResponseStatus": 200, "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87" }`,
			expectedLogs: func(t *testing.T, payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver")

				for idx, line := range strings.Split(payload, "\n") {
					lr := sl.LogRecords().AppendEmpty()

					require.NoError(t, lr.Attributes().FromRaw(map[string]any{
						"http_request.client_ip": fmt.Sprintf("89.163.253.%d", 200+idx),
					}))

					lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
					ts, _ := time.Parse(time.RFC3339, "2023-03-03T05:29:05Z")
					lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
					lr.SetSeverityNumber(plog.SeverityNumberInfo)
					lr.SetSeverityText(plog.SeverityNumberInfo.String())

					var log map[string]any
					err := json.Unmarshal([]byte(line), &log)
					require.NoError(t, err)

					payloadToExpectedBody(t, line, lr)
				}

				return logs
			},
		},

		{
			name:    "all fields",
			payload: `{"RayID":"7a1f7ad4df2f870a","EdgeStartTimestamp":"2023-03-03T05:29:06Z","CacheCacheStatus":"dynamic","CacheReserveUsed":false,"CacheResponseBytes":9247,"CacheResponseStatus":401,"CacheTieredFill":false,"ClientASN":20115,"ClientCountry":"us","ClientDeviceType":"desktop","ClientIP":"47.35.104.49","ClientIPClass":"noRecord","ClientMTLSAuthCertFingerprint":"","ClientMTLSAuthStatus":"unknown","ClientRegionCode":"MI","ClientRequestBytes":2667,"ClientRequestHost":"www.theburritobot2.com","ClientRequestMethod":"GET","ClientRequestPath":"/product/66VCHSJNUP","ClientRequestProtocol":"HTTP/2","ClientRequestReferer":"https://www.theburritobot2.com/","ClientRequestScheme":"https","ClientRequestSource":"eyeball","ClientRequestURI":"/product/66VCHSJNUP","ClientRequestUserAgent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36","ClientSSLCipher":"AEAD-AES128-GCM-SHA256","ClientSSLProtocol":"TLSv1.3","ClientSrcPort":49358,"ClientTCPRTTMs":18,"ClientXRequestedWith":"","ContentScanObjResults":[],"ContentScanObjTypes":[],"Cookies":{},"EdgeCFConnectingO2O":false,"EdgeColoCode":"ORD","EdgeColoID":398,"EdgeEndTimestamp":"2023-03-03T05:29:06Z","EdgePathingOp":"wl","EdgePathingSrc":"macro","EdgePathingStatus":"nr","EdgeRateLimitAction":"","EdgeRateLimitID":0,"EdgeRequestHost":"www.theburritobot2.com","EdgeResponseBodyBytes":1963,"EdgeResponseBytes":2301,"EdgeResponseCompressionRatio":2.54,"EdgeResponseContentType":"text/html","EdgeResponseStatus":401,"EdgeServerIP":"172.70.131.84","EdgeTimeToFirstByteMs":28,"FirewallMatchesActions":[],"FirewallMatchesRuleIDs":[],"FirewallMatchesSources":[],"OriginDNSResponseTimeMs":0,"OriginIP":"35.223.103.128","OriginRequestHeaderSendDurationMs":0,"OriginResponseBytes":0,"OriginResponseDurationMs":22,"OriginResponseHTTPExpires":"","OriginResponseHTTPLastModified":"","OriginResponseHeaderReceiveDurationMs":21,"OriginResponseStatus":401,"OriginResponseTime":22000000,"OriginSSLProtocol":"none","OriginTCPHandshakeDurationMs":0,"OriginTLSHandshakeDurationMs":0,"ParentRayID":"00","RequestHeaders":{},"ResponseHeaders":{},"SecurityLevel":"med","SmartRouteColoID":0,"UpperTierColoID":0,"WAFAction":"unknown","WAFAttackScore":0,"WAFFlags":"0","WAFMatchedVar":"","WAFProfile":"unknown","WAFRCEAttackScore":0,"WAFRuleID":"","WAFRuleMessage":"","WAFSQLiAttackScore":0,"WAFXSSAttackScore":0,"WorkerCPUTime":0,"WorkerStatus":"unknown","WorkerSubrequest":false,"WorkerSubrequestCount":0,"WorkerWallTimeUs":0,"ZoneName":"otlpdev.net"}`,
			expectedLogs: func(t *testing.T, payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()

				require.NoError(t, rl.Resource().Attributes().FromRaw(map[string]any{
					"cloudflare.zone": "otlpdev.net",
				}))

				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver")
				lr := sl.LogRecords().AppendEmpty()

				require.NoError(t, lr.Attributes().FromRaw(map[string]any{
					"http_request.client_ip": "47.35.104.49",
				}))

				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
				ts, _ := time.Parse(time.RFC3339, "2023-03-03T05:29:06Z")
				lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				lr.SetSeverityNumber(plog.SeverityNumberWarn)
				lr.SetSeverityText(plog.SeverityNumberWarn.String())

				var log map[string]any
				err := json.Unmarshal([]byte(payload), &log)
				require.NoError(t, err)

				payloadToExpectedBody(t, payload, lr)

				return logs
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recv := newReceiver(t, &Config{
				Logs: LogsConfig{
					Endpoint:       "localhost:0",
					TLS:            &configtls.ServerConfig{},
					TimestampField: "EdgeStartTimestamp",
					Attributes: map[string]string{
						"ClientIP": "http_request.client_ip",
					},
				},
			},
				&consumertest.LogsSink{},
			)
			var logs plog.Logs
			rawLogs, err := parsePayload([]byte(tc.payload))
			if err == nil {
				logs = recv.processLogs(pcommon.NewTimestampFromTime(time.Now()), rawLogs)
			}
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Nil(t, logs)
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, logs)
				require.NoError(t, plogtest.CompareLogs(tc.expectedLogs(t, tc.payload), logs, plogtest.IgnoreObservedTimestamp()))
			}
		})
	}
}

func payloadToExpectedBody(t *testing.T, payload string, lr plog.LogRecord) {
	var body map[string]any
	err := json.Unmarshal([]byte(payload), &body)
	require.NoError(t, err)
	require.NoError(t, lr.Body().SetEmptyMap().FromRaw(body))
}

func TestSeverityParsing(t *testing.T) {
	testCases := []struct {
		statusCode int64
		s          plog.SeverityNumber
	}{
		{
			statusCode: 200,
			s:          plog.SeverityNumberInfo,
		},
		{
			statusCode: 300,
			s:          plog.SeverityNumberInfo2,
		},
		{
			statusCode: 400,
			s:          plog.SeverityNumberWarn,
		},
		{
			statusCode: 501,
			s:          plog.SeverityNumberError,
		},
		{
			statusCode: 999,
			s:          plog.SeverityNumberUnspecified,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d -> %s", tc.statusCode, tc.s.String()), func(t *testing.T) {
			require.Equal(t, tc.s, severityFromStatusCode(tc.statusCode))
		})
	}
}

func TestHandleRequest(t *testing.T) {
	testCases := []struct {
		name               string
		cfg                *Config
		request            *http.Request
		expectedStatusCode int
		logExpected        bool
		consumerFailure    bool
		permanentFailure   bool // indicates a permanent error
	}{
		{
			name: "No secret provided",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`{"ClientIP": "127.0.0.1"}`)),
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "Invalid payload",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`{"ClientIP": "127.0.0.1"`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName): {"abc123"},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "Consumer fails",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`{"ClientIP": "127.0.0.1"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName): {"abc123"},
				},
			},
			logExpected:        false,
			consumerFailure:    true,
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		{
			name: "Consumer fails - permanent error",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`{"ClientIP": "127.0.0.1"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName): {"abc123"},
				},
			},
			logExpected:        false,
			consumerFailure:    true,
			permanentFailure:   true,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Request succeeds",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`{"ClientIP": "127.0.0.1", "MyTimestamp": "2023-03-03T05:29:06Z"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName): {"abc123"},
				},
			},
			logExpected:        true,
			consumerFailure:    false,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Request succeeds with gzip",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(gzippedMessage(`{"ClientIP": "127.0.0.1", "MyTimestamp": "2023-03-03T05:29:06Z"}`))),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName):   {"abc123"},
					textproto.CanonicalMIMEHeaderKey("Content-Encoding"): {"gzip"},
					textproto.CanonicalMIMEHeaderKey("Content-Type"):     {"text/plain; charset=utf-8"},
				},
			},
			logExpected:        true,
			consumerFailure:    false,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Request fails to unzip gzip",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`thisisnotvalidzippedcontent`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName):   {"abc123"},
					textproto.CanonicalMIMEHeaderKey("Content-Encoding"): {"gzip"},
					textproto.CanonicalMIMEHeaderKey("Content-Type"):     {"text/plain; charset=utf-8"},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "test message passes",
			request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
				Body:   io.NopCloser(bytes.NewBufferString(`test`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(secretHeaderName): {"abc123"},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var consumer consumer.Logs
			if tc.consumerFailure {
				consumer = consumertest.NewErr(errors.New("consumer failed"))
				if tc.permanentFailure {
					consumer = consumertest.NewErr(consumererror.NewPermanent(errors.New("consumer failed")))
				}
			} else {
				consumer = &consumertest.LogsSink{}
			}

			r := newReceiver(t, &Config{
				Logs: LogsConfig{
					Endpoint:       "localhost:0",
					Secret:         "abc123",
					TimestampField: "MyTimestamp",
					Attributes: map[string]string{
						"ClientIP": "http_request.client_ip",
					},
					TLS: &configtls.ServerConfig{},
				},
			},
				consumer,
			)

			rec := httptest.NewRecorder()
			r.handleRequest(rec, tc.request)

			assert.Equal(t, tc.expectedStatusCode, rec.Code, "Status codes are not equal")

			if !tc.consumerFailure {
				if tc.logExpected {
					assert.Equal(t, 1, consumer.(*consumertest.LogsSink).LogRecordCount(), "Did not receive log record")
				} else {
					assert.Equal(t, 0, consumer.(*consumertest.LogsSink).LogRecordCount(), "Received log record when it should have been dropped")
				}
			}
		})
	}
}

func TestEmptyAttributes(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name       string
		payload    string
		attributes map[string]string
	}{
		{
			name: "Empty Attributes Map",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }
{ "ClientIP" : "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }`,
			attributes: map[string]string{},
		},
		{
			name: "nil Attributes",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }
{ "ClientIP" : "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }`,
			attributes: nil,
		},
	}

	expectedLogs := func(t *testing.T, payload string) plog.Logs {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver")

		for idx, line := range strings.Split(payload, "\n") {
			lr := sl.LogRecords().AppendEmpty()

			require.NoError(t, lr.Attributes().FromRaw(map[string]any{
				"ClientIP":                    fmt.Sprintf("89.163.253.%d", 200+idx),
				"ClientRequestHost":           fmt.Sprintf("www.theburritobot%d.com", idx),
				"ClientRequestMethod":         "GET",
				"ClientRequestURI":            "/static/img/testimonial-hipster.png",
				"EdgeEndTimestamp":            "2023-03-03T05:30:05Z",
				"EdgeResponseBytes":           "69045",
				"EdgeResponseStatus":          "200",
				"EdgeStartTimestamp":          "2023-03-03T05:29:05Z",
				"RayID":                       "3a6050bcbe121a87",
				"RequestHeaders.Content_Type": "application/json",
			}))

			lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
			ts, err := time.Parse(time.RFC3339, "2023-03-03T05:29:05Z")
			require.NoError(t, err)
			lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			lr.SetSeverityNumber(plog.SeverityNumberInfo)
			lr.SetSeverityText(plog.SeverityNumberInfo.String())

			var log map[string]any
			err = json.Unmarshal([]byte(line), &log)
			require.NoError(t, err)

			payloadToExpectedBody(t, line, lr)
		}
		return logs
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recv := newReceiver(t, &Config{
				Logs: LogsConfig{
					Endpoint:       "localhost:0",
					TLS:            &configtls.ServerConfig{},
					TimestampField: "EdgeStartTimestamp",
					Attributes:     tc.attributes,
					Separator:      ".",
				},
			},
				&consumertest.LogsSink{},
			)
			var logs plog.Logs
			rawLogs, err := parsePayload([]byte(tc.payload))
			if err == nil {
				logs = recv.processLogs(pcommon.NewTimestampFromTime(time.Now()), rawLogs)
			}
			require.NoError(t, err)
			require.NotNil(t, logs)
			require.NoError(t, plogtest.CompareLogs(expectedLogs(t, tc.payload), logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestAttributesWithSeparator(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name       string
		payload    string
		attributes map[string]string
		separator  string
	}{
		{
			name: "Empty Attributes Map",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }
{ "ClientIP" : "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }`,
			attributes: map[string]string{},
			separator:  ".",
		},
		{
			name: "nil Attributes",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }
{ "ClientIP" : "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Content-Type": "application/json" } }`,
			attributes: nil,
			separator:  "_test_",
		},
	}

	expectedLogs := func(t *testing.T, payload string, separator string) plog.Logs {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver")

		for idx, line := range strings.Split(payload, "\n") {
			lr := sl.LogRecords().AppendEmpty()

			require.NoError(t, lr.Attributes().FromRaw(map[string]any{
				"ClientIP":            fmt.Sprintf("89.163.253.%d", 200+idx),
				"ClientRequestHost":   fmt.Sprintf("www.theburritobot%d.com", idx),
				"ClientRequestMethod": "GET",
				"ClientRequestURI":    "/static/img/testimonial-hipster.png",
				"EdgeEndTimestamp":    "2023-03-03T05:30:05Z",
				"EdgeResponseBytes":   "69045",
				"EdgeResponseStatus":  "200",
				"EdgeStartTimestamp":  "2023-03-03T05:29:05Z",
				"RayID":               "3a6050bcbe121a87",
				fmt.Sprintf("RequestHeaders%sContent_Type", separator): "application/json",
			}))

			lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
			ts, err := time.Parse(time.RFC3339, "2023-03-03T05:29:05Z")
			require.NoError(t, err)
			lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			lr.SetSeverityNumber(plog.SeverityNumberInfo)
			lr.SetSeverityText(plog.SeverityNumberInfo.String())

			var log map[string]any
			err = json.Unmarshal([]byte(line), &log)
			require.NoError(t, err)

			payloadToExpectedBody(t, line, lr)
		}
		return logs
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recv := newReceiver(t, &Config{
				Logs: LogsConfig{
					Endpoint:       "localhost:0",
					TLS:            &configtls.ServerConfig{},
					TimestampField: "EdgeStartTimestamp",
					Attributes:     tc.attributes,
					Separator:      tc.separator,
				},
			},
				&consumertest.LogsSink{},
			)
			var logs plog.Logs
			rawLogs, err := parsePayload([]byte(tc.payload))
			if err == nil {
				logs = recv.processLogs(pcommon.NewTimestampFromTime(time.Now()), rawLogs)
			}
			require.NoError(t, err)
			require.NotNil(t, logs)
			require.NoError(t, plogtest.CompareLogs(expectedLogs(t, tc.payload, tc.separator), logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestMultipleMapAttributes(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name       string
		payload    string
		attributes map[string]string
	}{
		{
			name: "Multi Map Attributes",
			payload: `{ "ClientIP": "89.163.253.200", "ClientRequestHost": "www.theburritobot0.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Cookie-new": { "x-m2-new": "sensitive" } } }
{ "ClientIP" : "89.163.253.201", "ClientRequestHost": "www.theburritobot1.com", "ClientRequestMethod": "GET", "ClientRequestURI": "/static/img/testimonial-hipster.png", "EdgeEndTimestamp": "2023-03-03T05:30:05Z", "EdgeResponseBytes": "69045", "EdgeResponseStatus": "200", "EdgeStartTimestamp": "2023-03-03T05:29:05Z", "RayID": "3a6050bcbe121a87", "RequestHeaders": { "Cookie-new": { "x-m2-new": "sensitive" } } }`,
			attributes: map[string]string{},
		},
	}

	expectedLogs := func(t *testing.T, payload string) plog.Logs {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver")

		for idx, line := range strings.Split(payload, "\n") {
			lr := sl.LogRecords().AppendEmpty()

			require.NoError(t, lr.Attributes().FromRaw(map[string]any{
				"ClientIP":                           fmt.Sprintf("89.163.253.%d", 200+idx),
				"ClientRequestHost":                  fmt.Sprintf("www.theburritobot%d.com", idx),
				"ClientRequestMethod":                "GET",
				"ClientRequestURI":                   "/static/img/testimonial-hipster.png",
				"EdgeEndTimestamp":                   "2023-03-03T05:30:05Z",
				"EdgeResponseBytes":                  "69045",
				"EdgeResponseStatus":                 "200",
				"EdgeStartTimestamp":                 "2023-03-03T05:29:05Z",
				"RayID":                              "3a6050bcbe121a87",
				"RequestHeaders.Cookie_new.x_m2_new": "sensitive",
			}))

			lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
			ts, err := time.Parse(time.RFC3339, "2023-03-03T05:29:05Z")
			require.NoError(t, err)
			lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			lr.SetSeverityNumber(plog.SeverityNumberInfo)
			lr.SetSeverityText(plog.SeverityNumberInfo.String())

			var log map[string]any
			err = json.Unmarshal([]byte(line), &log)
			require.NoError(t, err)

			payloadToExpectedBody(t, line, lr)
		}
		return logs
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recv := newReceiver(t, &Config{
				Logs: LogsConfig{
					Endpoint:       "localhost:0",
					TLS:            &configtls.ServerConfig{},
					TimestampField: "EdgeStartTimestamp",
					Attributes:     tc.attributes,
					Separator:      ".",
				},
			},
				&consumertest.LogsSink{},
			)
			var logs plog.Logs
			rawLogs, err := parsePayload([]byte(tc.payload))
			if err == nil {
				logs = recv.processLogs(pcommon.NewTimestampFromTime(time.Now()), rawLogs)
			}
			require.NoError(t, err)
			require.NotNil(t, logs)
			require.NoError(t, plogtest.CompareLogs(expectedLogs(t, tc.payload), logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func gzippedMessage(message string) string {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write([]byte(message))
	if err != nil {
		panic(err)
	}
	w.Close()
	return b.String()
}

func newReceiver(t *testing.T, cfg *Config, nextConsumer consumer.Logs) *logsReceiver {
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = zaptest.NewLogger(t)
	r, err := newLogsReceiver(set, cfg, nextConsumer)
	require.NoError(t, err)
	return r
}
