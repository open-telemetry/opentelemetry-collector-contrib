// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/storage"
	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadatatest"
)

const (
	emulatorBucket = "emulator-bucket"
	emulatorObject = "emulator-object"

	dummyLog = `{"log": "test"}`
)

func TestLoadEncodingExtension(t *testing.T) {
	encodingExt := mockEncodingExtension{}
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID("test_fail"):    nil,
			component.MustNewID("test_succeed"): encodingExt,
		},
	}

	_, err := loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.ID{}, "test")
	require.ErrorContains(t, err, `extension "" not found`)

	_, err = loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.MustNewID("test_fail"), "test")
	require.ErrorContains(t, err, `extension "test_fail" is not a test unmarshaler`)

	res, err := loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.MustNewID("test_succeed"), "test")
	require.NoError(t, err)
	require.Equal(t, encodingExt, res)
}

func TestHandlePubSubPushRequest_Logs(t *testing.T) {
	tests := map[string]struct {
		request         io.Reader
		consumeNext     func(context.Context, plog.Logs) error
		expectedErr     string
		includeMetadata bool
		isRetryable     bool
	}{
		"invalid request structure": {
			request:     strings.NewReader("invalid"),
			expectedErr: "failed to decode Pub/Sub request",
		},
		"invalid consume - retryable": {
			request: newTestRequest(t, nil, []byte(dummyLog), ""),
			consumeNext: func(context.Context, plog.Logs) error {
				return errors.New("failed")
			},
			expectedErr: "failed to consume unmarshalled request",
			isRetryable: true,
		},
		"invalid consume - permanent": {
			request: newTestRequest(t, nil, []byte(dummyLog), ""),
			consumeNext: func(context.Context, plog.Logs) error {
				return consumererror.NewPermanent(errors.New("failed"))
			},
			expectedErr: "failed to consume unmarshalled request",
		},
		"valid message data": {
			request: newTestRequest(t, nil, []byte(dummyLog), "projects/test/subscriptions/test"),
			consumeNext: func(ctx context.Context, _ plog.Logs) error {
				clCtx := client.FromContext(ctx)
				require.Empty(t, clCtx.Metadata.Get(subscriptionMetadataKey))
				return nil
			},
		},
		"valid message data - include metadata": {
			request: newTestRequest(t, nil, []byte(dummyLog), "projects/test/subscriptions/test"),
			consumeNext: func(ctx context.Context, _ plog.Logs) error {
				clCtx := client.FromContext(ctx)
				require.Equal(t, []string{"projects/test/subscriptions/test"}, clCtx.Metadata.Get(subscriptionMetadataKey))
				return nil
			},
			includeMetadata: true,
		},
		"storage notification - missing object key": {
			request: newTestRequest(t, map[string]string{
				bucketIDKey: emulatorBucket,
			}, nil, ""),
			expectedErr: "missing objectId attribute",
		},
		"storage notification - missing event type key": {
			request: newTestRequest(t, map[string]string{
				bucketIDKey: emulatorBucket,
				objectIDKey: emulatorObject,
			}, nil, ""),
			expectedErr: "missing eventType attribute",
		},
		"storage notification - not file created event type": {
			request: newTestRequest(t, map[string]string{
				bucketIDKey:  emulatorBucket,
				objectIDKey:  emulatorObject,
				eventTypeKey: "not-file-created",
			}, nil, ""),
			consumeNext: func(context.Context, plog.Logs) error {
				return errors.New("storage notification was ignored, this error should not be the result")
			},
		},
		"storage notification - get content from bucket object": {
			request: newTestRequest(t, map[string]string{
				bucketIDKey:  emulatorBucket,
				objectIDKey:  emulatorObject,
				eventTypeKey: eventObjectFinalize,
			}, nil, ""),
			consumeNext: func(ctx context.Context, _ plog.Logs) error {
				cl := client.FromContext(ctx)
				require.Equal(t, []string{emulatorBucket}, cl.Metadata.Get(bucketMetadataKey))
				require.Equal(t, []string{emulatorObject}, cl.Metadata.Get(objectMetadataKey))
				return nil
			},
			includeMetadata: true,
		},
	}

	pubSubReceiver := newTestPubSubPushReceiver(t, &Config{})
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := handlePubSubPushRequest(
				t.Context(),
				test.request,
				mockEncodingExtension{}.UnmarshalLogs,
				test.consumeNext,
				pubSubReceiver.storageClient,
				test.includeMetadata,
				nil,
			)

			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				require.Equal(t, test.isRetryable, !consumererror.IsPermanent(err))
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStartShutdown(t *testing.T) {
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID("test"): mockEncodingExtension{},
		},
	}

	startTestStorageEmulator(t)

	tests := map[string]struct {
		pubSubReceiver *pubSubPushReceiver
		expectedErr    string
	}{
		"invalid_encoding": {
			pubSubReceiver: &pubSubPushReceiver{
				cfg: &Config{
					Encoding: component.MustNewID("fails"),
				},
				nextLogs: consumertest.NewNop(),
			},
			expectedErr: "failed to load encoding extension",
		},
		"valid": {
			pubSubReceiver: &pubSubPushReceiver{
				cfg: &Config{
					Encoding:     component.MustNewID("test"),
					ServerConfig: confighttp.NewDefaultServerConfig(),
				},
				settings: receivertest.NewNopSettings(metadata.Type),
				nextLogs: consumertest.NewNop(),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.pubSubReceiver.Start(t.Context(), mHost)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			err = test.pubSubReceiver.Shutdown(t.Context())
			require.NoError(t, err)
		})
	}
}

func TestTelemetry(t *testing.T) {
	p := newTestPubSubPushReceiver(t, &Config{})
	tel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	p.telemetryBuilder = tb

	// We will let the function to unmarshal logs hang so we can test that the
	// active requests increases. Once we are sure of it, we let the function
	// return and make sure the active requests went down back to 0.
	done := make(chan bool, 1)
	started := make(chan bool, 1)
	mux := http.NewServeMux()
	addHandlerFunc(
		p.telemetryBuilder,
		mux,
		"/",
		func(_ []byte) (plog.Logs, error) {
			started <- true
			<-done
			return plog.Logs{}, nil
		},
		p.nextLogs.ConsumeLogs,
		p.storageClient,
		false,
		p.settings.Logger,
	)
	server := httptest.NewServer(mux)
	defer server.Close()

	// send long directly to Pub/Sub

	request := newTestRequest(t, map[string]string{}, []byte(dummyLog), "")

	var wg sync.WaitGroup
	makeRequest := func() {
		wg.Go(func() {
			resp, err := http.Post(
				server.URL,
				"application/json",
				request,
			)
			assert.NoError(t, err)
			defer func() {
				errBody := resp.Body.Close()
				assert.NoError(t, errBody)
			}()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
	pauseRequest := func() {
		<-started
	}
	stopRequest := func() {
		done <- true
		wg.Wait()
	}

	makeRequest()
	pauseRequest() // so we can test active requests

	metadatatest.AssertEqualHTTPServerRequestActiveCount(t, tel, []metricdata.DataPoint[int64]{{
		Value:      1,
		Attributes: attribute.NewSet(),
	}}, metricdatatest.IgnoreTimestamp())

	stopRequest()

	metadatatest.AssertEqualHTTPServerRequestActiveCount(t, tel, []metricdata.DataPoint[int64]{{
		Value:      0,
		Attributes: attribute.NewSet(),
	}}, metricdatatest.IgnoreTimestamp())

	metadatatest.AssertEqualHTTPServerRequestDuration(t, tel, []metricdata.HistogramDataPoint[float64]{{
		Attributes: attribute.NewSet(
			attribute.Int("http.response.status_code", http.StatusOK),
		),
	}}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())

	metadatatest.AssertEqualGcpPubsubInputUncompressedSize(t, tel, []metricdata.HistogramDataPoint[float64]{{
		// no attributes, since it comes directly from Pub/Sub
		Attributes: attribute.NewSet(),
	}}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())

	// send an event notification to Pub/Sub from placing log in bucket

	request = newTestRequest(t, map[string]string{
		bucketIDKey:  emulatorBucket,
		objectIDKey:  emulatorObject,
		eventTypeKey: eventObjectFinalize,
	}, nil, "")

	makeRequest()
	stopRequest()

	metadatatest.AssertEqualGcpPubsubInputUncompressedSize(t, tel, []metricdata.HistogramDataPoint[float64]{
		// log from first request went directly to Pub/Sub
		{Attributes: attribute.NewSet()},
		// log from last request went to emulator bucket
		{Attributes: attribute.NewSet(attribute.String(bucketNameAttr, emulatorBucket))},
	}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

type mockEncodingExtension struct {
	extension.Extension
}

var _ encoding.LogsUnmarshalerExtension = (*mockEncodingExtension)(nil)

func (mockEncodingExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.Logs{}, nil
}

func newTestRequest(t *testing.T, attrs map[string]string, data []byte, subscription string) io.Reader {
	m := pubSubPushRequest{
		Message: pubSubPushMessage{
			Attributes: attrs,
			Data:       data,
		},
		Subscription: subscription,
	}
	request, err := gojson.Marshal(m)
	require.NoError(t, err)
	return bytes.NewReader(request)
}

func newTestPubSubPushReceiver(t *testing.T, cfg *Config) *pubSubPushReceiver {
	startTestStorageEmulator(t) // we start the emulator so the storage client creation is successful
	r, err := newPubSubPushReceiver(cfg, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	cl, err := storage.NewClient(t.Context())
	require.NoError(t, err)
	r.storageClient = cl
	return r
}

func startTestStorageEmulator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodPath := r.Method + " " + r.URL.Path
		switch methodPath {
		case fmt.Sprintf("GET /%s/%s", emulatorBucket, emulatorObject):
			w.WriteHeader(http.StatusOK)
		case fmt.Sprintf("POST /%s/%s", emulatorBucket, emulatorObject):
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("Unexpected request: %s", methodPath)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(func() {
		server.Close()
	})

	t.Setenv("STORAGE_EMULATOR_HOST", server.Listener.Addr().String())
}
