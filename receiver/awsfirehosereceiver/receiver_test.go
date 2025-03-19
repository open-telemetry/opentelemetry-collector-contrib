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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
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

func (*nopFirehoseConsumer) Start(context.Context, component.Host) error {
	return nil
}

func (nfc *nopFirehoseConsumer) Consume(context.Context, nextRecordFunc, map[string]string) (int, error) {
	return nfc.statusCode, nfc.err
}

type firehoseConsumerFunc func(context.Context, nextRecordFunc, map[string]string) (int, error)

func (firehoseConsumerFunc) Start(context.Context, component.Host) error {
	return nil
}

func (f firehoseConsumerFunc) Consume(ctx context.Context, next nextRecordFunc, common map[string]string) (int, error) {
	return f(ctx, next, common)
}

func TestStart(t *testing.T) {
	testCases := map[string]struct {
		host    component.Host
		wantErr error
	}{
		"WithHost": {
			host: componenttest.NewNopHost(),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{}
			ctx := context.TODO()
			r := testFirehoseReceiver(cfg, &nopFirehoseConsumer{})
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
			ServerConfig: confighttp.ServerConfig{
				Endpoint: listener.Addr().String(),
			},
		}
		ctx := context.TODO()
		r := testFirehoseReceiver(cfg, &nopFirehoseConsumer{})
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
		AccessKey: testFirehoseAccessKey,
	}
	var noRecords []firehoseRecord
	testCases := map[string]struct {
		headers          map[string]string
		commonAttributes map[string]string
		body             any
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
		"WithNoAccessKey": {
			headers: map[string]string{
				headerFirehoseAccessKey: "",
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
			consumer: firehoseConsumerFunc(func(_ context.Context, next nextRecordFunc, _ map[string]string) (int, error) {
				if _, err := next(); err != nil {
					return http.StatusBadRequest, err
				}
				return http.StatusOK, nil
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

			request := newTestRequest(body)
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

// TestFirehoseRequestInvalidJSON is tested separately from TestFirehoseRequest
// because the error message is highly dependent on the JSON decoding library used,
// so we don't do an exact match.
func TestFirehoseRequestInvalidJSON(t *testing.T) {
	consumer := newNopFirehoseConsumer(http.StatusOK, nil)
	r := testFirehoseReceiver(&Config{}, consumer)

	got := httptest.NewRecorder()
	r.ServeHTTP(got, newTestRequest([]byte("{ test: ")))
	require.Equal(t, http.StatusBadRequest, got.Code)

	var gotResponse firehoseResponse
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &gotResponse))
	require.Equal(t, testFirehoseRequestID, gotResponse.RequestID)
	require.Regexp(t, gotResponse.ErrorMessage, `awsfirehosereceiver\.firehoseRequest\.ReadStringAsSlice: expects .*`)
}

// testFirehoseReceiver is a convenience function for creating a test firehoseReceiver
func testFirehoseReceiver(config *Config, consumer firehoseConsumer) *firehoseReceiver {
	return &firehoseReceiver{
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   config,
		consumer: consumer,
	}
}

func newTestRequest(requestBody []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(requestBody))
	request.Header.Set(headerContentType, "application/json")
	request.Header.Set(headerContentLength, strconv.Itoa(len(requestBody)))
	request.Header.Set(headerFirehoseRequestID, testFirehoseRequestID)
	request.Header.Set(headerFirehoseAccessKey, testFirehoseAccessKey)
	return request
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

func newNextRecordFunc(records [][]byte) nextRecordFunc {
	return func() ([]byte, error) {
		if len(records) == 0 {
			return nil, io.EOF
		}
		next := records[0]
		records = records[1:]
		return next, nil
	}
}

type hostWithExtensions struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type plogUnmarshalerExtension struct {
	logs plog.Logs
}

func (e plogUnmarshalerExtension) Start(context.Context, component.Host) error {
	return nil
}

func (e plogUnmarshalerExtension) Shutdown(context.Context) error {
	return nil
}

func (e plogUnmarshalerExtension) UnmarshalLogs([]byte) (plog.Logs, error) {
	return e.logs, nil
}

type pmetricUnmarshalerExtension struct {
	metrics pmetric.Metrics
}

func (e pmetricUnmarshalerExtension) Start(context.Context, component.Host) error {
	return nil
}

func (e pmetricUnmarshalerExtension) Shutdown(context.Context) error {
	return nil
}

func (e pmetricUnmarshalerExtension) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	return e.metrics, nil
}
