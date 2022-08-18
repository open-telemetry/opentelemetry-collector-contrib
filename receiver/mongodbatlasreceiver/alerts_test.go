// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbatlasreceiver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func TestPayloadToLogRecord(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name         string
		payload      string
		expectedLogs func(string) plog.Logs
		expectedErr  string
	}{
		{
			name: "minimal record",
			payload: `{
				"created": "2022-06-03T22:30:31Z",
				"groupId": "some-group-id",
				"humanReadable": "Some event happened",
				"alertConfigId": "123",
				"eventTypeName": "EVENT",
				"id": "some-id",
				"updated": "2022-06-03T22:30:31Z",
				"status": "STATUS"
			}`,
			expectedLogs: func(payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

				pcommon.NewMapFromRaw(
					map[string]interface{}{
						"mongodbatlas.group.id":        "some-group-id",
						"mongodbatlas.alert.config.id": "123",
					},
				).CopyTo(rl.Resource().Attributes())

				pcommon.NewMapFromRaw(
					map[string]interface{}{
						"created":      "2022-06-03T22:30:31Z",
						"message":      "Some event happened",
						"event.domain": "mongodbatlas",
						"event.name":   "EVENT",
						"updated":      "2022-06-03T22:30:31Z",
						"status":       "STATUS",
						"id":           "some-id",
					},
				).CopyTo(lr.Attributes())

				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
				lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2022, time.June, 3, 22, 30, 31, 0, time.UTC)))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)

				lr.Body().SetStringVal(payload)

				return logs
			},
		},

		{
			name: "optional fields",
			payload: `{
				"created": "2022-06-03T22:30:31Z",
				"groupId": "some-group-id",
				"humanReadable": "Some event happened",
				"alertConfigId": "123",
				"eventTypeName": "EVENT",
				"id": "some-id",
				"updated": "2022-06-03T22:30:35Z",
				"status": "STATUS",
				"replicaSetName": "replica-set",
				"metricName": "my-metric",
				"typeName": "type-name",
				"clusterName": "cluster-name",
				"userAlias": "user-alias",
				"lastNotified": "2022-06-03T22:30:33Z",
				"resolved": "2022-06-03T22:30:34Z",
				"acknowledgementComment": "Scheduled maintenance",
				"acknowledgingUsername": "devops",
				"acknowledgedUntil": "2022-06-03T22:32:34Z",
				"hostnameAndPort": "my-host.mongodb.com:4923",
				"currentValue": {
					"number": 14,
					"units": "RAW"
				}
			}`,
			expectedLogs: func(payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

				pcommon.NewMapFromRaw(
					map[string]interface{}{
						"mongodbatlas.group.id":         "some-group-id",
						"mongodbatlas.alert.config.id":  "123",
						"mongodbatlas.cluster.name":     "cluster-name",
						"mongodbatlas.replica_set.name": "replica-set",
					},
				).CopyTo(rl.Resource().Attributes())

				pcommon.NewMapFromRaw(
					map[string]interface{}{
						"acknowledgement.comment":  "Scheduled maintenance",
						"acknowledgement.until":    "2022-06-03T22:32:34Z",
						"acknowledgement.username": "devops",
						"created":                  "2022-06-03T22:30:31Z",
						"event.name":               "EVENT",
						"event.domain":             "mongodbatlas",
						"id":                       "some-id",
						"last_notified":            "2022-06-03T22:30:33Z",
						"message":                  "Some event happened",
						"metric.name":              "my-metric",
						"metric.units":             "RAW",
						"metric.value":             float64(14),
						"net.peer.name":            "my-host.mongodb.com",
						"net.peer.port":            4923,
						"resolved":                 "2022-06-03T22:30:34Z",
						"status":                   "STATUS",
						"type_name":                "type-name",
						"updated":                  "2022-06-03T22:30:35Z",
						"user_alias":               "user-alias",
					},
				).CopyTo(lr.Attributes())

				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
				lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2022, time.June, 3, 22, 30, 35, 0, time.UTC)))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)

				lr.Body().SetStringVal(payload)

				return logs
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := payloadToLogs(now, []byte(tc.payload))
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Nil(t, logs)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, logs)
				require.NoError(t, compareLogs(tc.expectedLogs(tc.payload), *logs))
			}
		})
	}
}

func TestSeverityFromAlert(t *testing.T) {
	testCases := []struct {
		alert model.Alert
		s     plog.SeverityNumber
	}{
		{
			alert: model.Alert{Status: "OPEN"},
			s:     plog.SeverityNumberWarn,
		},
		{
			alert: model.Alert{Status: "TRACKING"},
			s:     plog.SeverityNumberInfo,
		},
		{
			alert: model.Alert{Status: "CLOSED"},
			s:     plog.SeverityNumberInfo,
		},
		{
			alert: model.Alert{Status: "INFORMATIONAL"},
			s:     plog.SeverityNumberInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s -> %s", tc.alert.Status, tc.s.String()), func(t *testing.T) {
			s := severityFromAlert(tc.alert)
			require.Equal(t, tc.s, s)
		})
	}
}

func TestVerifyHMACSignature(t *testing.T) {
	testCases := []struct {
		name            string
		secret          string
		payload         []byte
		signatureHeader string
		expectedErr     string
	}{
		{
			name:            "Signature matches",
			secret:          "some_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "n5O6j3X1o1iYgFLcbjjMHBIYkFU=",
		},
		{
			name:            "Signature does not match",
			secret:          "a_different_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "n5O6j3X1o1iYgFLcbjjMHBIYkFU=",
			expectedErr:     "calculated signature does not equal header signature",
		},
		{
			name:            "Signature is not valid base64",
			secret:          "a_different_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "%asd82kmnc~`",
			expectedErr:     "illegal base64 data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyHMACSignature(tc.secret, tc.payload, tc.signatureHeader)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

		})
	}
}

func TestHandleRequest(t *testing.T) {
	testCases := []struct {
		name               string
		request            *http.Request
		expectedStatusCode int
		logExpected        bool
		consumerFailure    bool
	}{
		{
			name: "No ContentLength set",
			request: &http.Request{
				ContentLength: -1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusLengthRequired,
		},
		{
			name: "ContentLength too large",
			request: &http.Request{
				ContentLength: maxContentLength + 1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "ContentLength is incorrect for payload",
			request: &http.Request{
				ContentLength: 1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "ContentLength is larger than actual payload",
			request: &http.Request{
				ContentLength: 32,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "No HMAC signature",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Incorrect HMAC signature",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"n5O6j3X1o1iYgFLcbjjMHBIYkFU="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Invalid payload",
			request: &http.Request{
				ContentLength: 14,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"P8OMdoVqNNE9iTFccTuwTY/J7vQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Consumer fails",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    true,
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "Request succeeds",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        true,
			consumerFailure:    false,
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var consumer consumer.Logs
			if tc.consumerFailure {
				consumer = consumertest.NewErr(errors.New("consumer failed"))
			} else {
				consumer = &consumertest.LogsSink{}
			}

			ar, err := newAlertsReceiver(zaptest.NewLogger(t), AlertConfig{
				Secret: "some_secret",
			}, consumer)

			require.NoError(t, err, "Failed to create alerts receiver")

			rec := httptest.NewRecorder()
			ar.handleRequest(rec, tc.request)

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

func compareLogs(expected, actual plog.Logs) error {
	if expected.ResourceLogs().Len() != actual.ResourceLogs().Len() {
		return fmt.Errorf("amount of ResourceLogs between Logs are not equal (expected: %d, actual: %d)",
			expected.ResourceLogs().Len(),
			actual.ResourceLogs().Len())
	}

	for i := 0; i < expected.ResourceLogs().Len(); i++ {
		err := compareResourceLogs(expected.ResourceLogs().At(i), actual.ResourceLogs().At(i))
		if err != nil {
			return fmt.Errorf("resource logs at index %d: %w", i, err)
		}
	}

	return nil
}

func compareResourceLogs(expected, actual plog.ResourceLogs) error {
	if expected.SchemaUrl() != actual.SchemaUrl() {
		return fmt.Errorf("resource logs SchemaUrl doesn't match (expected: %s, actual: %s)",
			expected.SchemaUrl(),
			actual.SchemaUrl())
	}

	if !reflect.DeepEqual(expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()) {
		return fmt.Errorf("resource logs Attributes doesn't match (expected: %+v, actual: %+v)",
			expected.Resource().Attributes().AsRaw(),
			actual.Resource().Attributes().AsRaw())
	}

	if expected.Resource().DroppedAttributesCount() != actual.Resource().DroppedAttributesCount() {
		return fmt.Errorf("resource logs DroppedAttributesCount doesn't match (expected: %d, actual: %d)",
			expected.Resource().DroppedAttributesCount(),
			actual.Resource().DroppedAttributesCount())
	}

	if expected.ScopeLogs().Len() != actual.ScopeLogs().Len() {
		return fmt.Errorf("amount of ScopeLogs between ResourceLogs are not equal (expected: %d, actual: %d)",
			expected.ScopeLogs().Len(),
			actual.ScopeLogs().Len())
	}

	for i := 0; i < expected.ScopeLogs().Len(); i++ {
		err := compareScopeLogs(expected.ScopeLogs().At(i), actual.ScopeLogs().At(i))
		if err != nil {
			return fmt.Errorf("scope logs at index %d: %w", i, err)
		}
	}

	return nil
}

func compareScopeLogs(expected, actual plog.ScopeLogs) error {
	if expected.SchemaUrl() != actual.SchemaUrl() {
		return fmt.Errorf("log scope SchemaUrl doesn't match (expected: %s, actual: %s)",
			expected.SchemaUrl(),
			actual.SchemaUrl())
	}

	if expected.Scope().Name() != actual.Scope().Name() {
		return fmt.Errorf("log scope Name doesn't match (expected: %s, actual: %s)",
			expected.Scope().Name(),
			actual.Scope().Name())
	}

	if expected.Scope().Version() != actual.Scope().Version() {
		return fmt.Errorf("log scope Version doesn't match (expected: %s, actual: %s)",
			expected.Scope().Version(),
			actual.Scope().Version())
	}

	if expected.LogRecords().Len() != actual.LogRecords().Len() {
		return fmt.Errorf("amount of log records between ScopeLogs are not equal (expected: %d, actual: %d)",
			expected.LogRecords().Len(),
			actual.LogRecords().Len())
	}

	for i := 0; i < expected.LogRecords().Len(); i++ {
		err := compareLogRecord(expected.LogRecords().At(i), actual.LogRecords().At(i))
		if err != nil {
			return fmt.Errorf("log record at index %d: %w", i, err)
		}
	}

	return nil
}

func compareLogRecord(expected, actual plog.LogRecord) error {
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("log record Flags doesn't match (expected: %d, actual: %d)",
			expected.Flags(),
			actual.Flags())
	}

	if expected.DroppedAttributesCount() != actual.DroppedAttributesCount() {
		return fmt.Errorf("log record DroppedAttributesCount doesn't match (expected: %d, actual: %d)",
			expected.DroppedAttributesCount(),
			actual.DroppedAttributesCount())
	}

	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("log record Timestamp doesn't match (expected: %d, actual: %d)",
			expected.Timestamp(),
			actual.Timestamp())
	}

	if expected.SeverityNumber() != actual.SeverityNumber() {
		return fmt.Errorf("log record SeverityNumber doesn't match (expected: %d, actual: %d)",
			expected.SeverityNumber(),
			actual.SeverityNumber())
	}

	if expected.SeverityText() != actual.SeverityText() {
		return fmt.Errorf("log record SeverityText doesn't match (expected: %s, actual: %s)",
			expected.SeverityText(),
			actual.SeverityText())
	}

	if expected.TraceID() != actual.TraceID() {
		return fmt.Errorf("log record TraceID doesn't match (expected: %d, actual: %d)",
			expected.TraceID(),
			actual.TraceID())
	}

	if expected.SpanID() != actual.SpanID() {
		return fmt.Errorf("log record SpanID doesn't match (expected: %d, actual: %d)",
			expected.SpanID(),
			actual.SpanID())
	}

	if !expected.Body().Equal(actual.Body()) {
		return fmt.Errorf("log record Body doesn't match (expected: %s, actual: %s)",
			expected.Body().AsString(),
			actual.Body().AsString())
	}

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("log record Attributes doesn't match (expected: %#v, actual: %#v)",
			expected.Attributes().AsRaw(),
			actual.Attributes().AsRaw())
	}

	return nil
}
