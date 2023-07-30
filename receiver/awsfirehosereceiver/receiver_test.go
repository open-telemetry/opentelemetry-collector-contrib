// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

const (
	testFirehoseRequestID = "firehose-request-id"
	testFirehoseAccessKey = "firehose-access-key"
)

type nopFirehoseConsumer struct {
	statusCode int
	err        error
}

var _ firehoseConsumer = (*nopFirehoseConsumer)(nil)

func newNopFirehoseConsumer(statusCode int, err error) *nopFirehoseConsumer {
	return &nopFirehoseConsumer{statusCode, err}
}

func (nfc *nopFirehoseConsumer) Consume(context.Context, [][]byte, map[string]string) (int, error) {
	return nfc.statusCode, nfc.err
}

func TestStart(t *testing.T) {
	testCases := map[string]struct {
		host    component.Host
		wantErr error
	}{
		"WithoutHost": {
			wantErr: errMissingHost,
		},
		"WithHost": {
			host: componenttest.NewNopHost(),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				RecordType: defaultRecordType,
			}
			ctx := context.TODO()
			r := testFirehoseReceiver(cfg, nil)
			got := r.Start(ctx, testCase.host)
			require.Equal(t, testCase.wantErr, got)
			if r.server != nil {
				require.NoError(t, r.Shutdown(ctx))
			}
		})
	}
	t.Run("WithPortTaken", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, listener.Close())
		})
		cfg := &Config{
			RecordType: defaultRecordType,
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: listener.Addr().String(),
			},
		}
		ctx := context.TODO()
		r := testFirehoseReceiver(cfg, nil)
		got := r.Start(ctx, componenttest.NewNopHost())
		require.Error(t, got)
		if r.server != nil {
			require.NoError(t, r.Shutdown(ctx))
		}
	})
}

func TestFirehoseRequest(t *testing.T) {
	defaultConsumer := newNopFirehoseConsumer(http.StatusOK, nil)
	firehoseConsumerErr := errors.New("firehose consumer error")
	cfg := &Config{
		RecordType: defaultRecordType,
		AccessKey:  testFirehoseAccessKey,
	}
	var noRecords []firehoseRecord
	testCases := map[string]struct {
		headers          map[string]string
		commonAttributes map[string]string
		body             interface{}
		consumer         firehoseConsumer
		wantStatusCode   int
		wantErr          error
	}{
		"WithoutRequestId/Header": {
			headers: map[string]string{
				headerFirehoseRequestID: "",
			},
			body:           testFirehoseRequest(testFirehoseRequestID, noRecords),
			wantStatusCode: http.StatusBadRequest,
			wantErr:        errInHeaderMissingRequestID,
		},
		"WithDifferentAccessKey": {
			headers: map[string]string{
				headerFirehoseAccessKey: "test",
			},
			body:           testFirehoseRequest(testFirehoseRequestID, noRecords),
			wantStatusCode: http.StatusUnauthorized,
			wantErr:        errInvalidAccessKey,
		},
		"WithoutRequestId/Body": {
			headers: map[string]string{
				headerFirehoseRequestID: testFirehoseRequestID,
			},
			body:           testFirehoseRequest("", noRecords),
			wantStatusCode: http.StatusBadRequest,
			wantErr:        errInBodyMissingRequestID,
		},
		"WithDifferentRequestIds": {
			headers: map[string]string{
				headerFirehoseRequestID: testFirehoseRequestID,
			},
			body:           testFirehoseRequest("otherId", noRecords),
			wantStatusCode: http.StatusBadRequest,
			wantErr:        errInBodyDiffRequestID,
		},
		"WithInvalidBody": {
			body:           "{ test: ",
			wantStatusCode: http.StatusBadRequest,
			wantErr:        errors.New("json: cannot unmarshal string into Go value of type awsfirehosereceiver.firehoseRequest"),
		},
		"WithNoRecords": {
			body:           testFirehoseRequest(testFirehoseRequestID, noRecords),
			wantStatusCode: http.StatusOK,
		},
		"WithFirehoseConsumerError": {
			body:           testFirehoseRequest(testFirehoseRequestID, noRecords),
			consumer:       newNopFirehoseConsumer(http.StatusInternalServerError, firehoseConsumerErr),
			wantStatusCode: http.StatusInternalServerError,
			wantErr:        firehoseConsumerErr,
		},
		"WithCorruptBase64Records": {
			body: testFirehoseRequest(testFirehoseRequestID, []firehoseRecord{
				{Data: "XXXXXaGVsbG8="},
			}),
			wantStatusCode: http.StatusBadRequest,
			wantErr:        fmt.Errorf("unable to base64 decode the record at index 0: %w", base64.CorruptInputError(12)),
		},
		"WithValidRecords": {
			body: testFirehoseRequest(testFirehoseRequestID, []firehoseRecord{
				testFirehoseRecord("test"),
			}),
			wantStatusCode: http.StatusOK,
		},
		"WithValidRecords/CommonAttributes": {
			body: testFirehoseRequest(testFirehoseRequestID, []firehoseRecord{
				testFirehoseRecord("test"),
			}),
			commonAttributes: map[string]string{
				"TestAttribute": "common",
			},
			wantStatusCode: http.StatusOK,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(testCase.body)
			require.NoError(t, err)

			requestBody := bytes.NewBuffer(body)

			request := httptest.NewRequest("POST", "/", requestBody)
			request.Header.Set(headerContentType, "application/json")
			request.Header.Set(headerContentLength, fmt.Sprintf("%d", requestBody.Len()))
			request.Header.Set(headerFirehoseRequestID, testFirehoseRequestID)
			request.Header.Set(headerFirehoseAccessKey, testFirehoseAccessKey)
			if testCase.headers != nil {
				for k, v := range testCase.headers {
					request.Header.Set(k, v)
				}
			}
			if testCase.commonAttributes != nil {
				attrs, err := json.Marshal(firehoseCommonAttributes{
					CommonAttributes: testCase.commonAttributes,
				})
				require.NoError(t, err)
				request.Header.Set(headerFirehoseCommonAttributes, string(attrs))
			}

			consumer := testCase.consumer
			if consumer == nil {
				consumer = defaultConsumer
			}
			r := testFirehoseReceiver(cfg, consumer)

			got := httptest.NewRecorder()
			r.ServeHTTP(got, request)

			require.Equal(t, testCase.wantStatusCode, got.Code)
			var gotResponse firehoseResponse
			require.NoError(t, json.Unmarshal(got.Body.Bytes(), &gotResponse))
			require.Equal(t, request.Header.Get(headerFirehoseRequestID), gotResponse.RequestID)
			if testCase.wantErr != nil {
				require.Equal(t, testCase.wantErr.Error(), gotResponse.ErrorMessage)
			} else {
				require.Empty(t, gotResponse.ErrorMessage)
			}
		})
	}
}

// testFirehoseReceiver is a convenience function for creating a test firehoseReceiver
func testFirehoseReceiver(config *Config, consumer firehoseConsumer) *firehoseReceiver {
	return &firehoseReceiver{
		settings: receivertest.NewNopCreateSettings(),
		config:   config,
		consumer: consumer,
	}
}

func testFirehoseRequest(requestID string, records []firehoseRecord) firehoseRequest {
	return firehoseRequest{
		RequestID: requestID,
		Timestamp: time.Now().UnixMilli(),
		Records:   records,
	}
}

func testFirehoseRecord(data string) firehoseRecord {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	return firehoseRecord{Data: encoded}
}
