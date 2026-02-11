// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"
)

func TestComponentStatus(t *testing.T) {
	statusChan := make(chan *componentstatus.Event, 1)
	reporter := &testStatusReporter{statusChan}
	esLogger := clientLogger{
		Logger:          zap.New(nil),
		logRequestBody:  false,
		logResponseBody: false,
		componentHost:   reporter,
	}

	// Pass in an error and make sure it's sent to the component status reporter
	_ = esLogger.LogRoundTrip(nil, nil, io.EOF, time.Now(), 0)
	select {
	case event := <-statusChan:
		assert.ErrorIs(t, event.Err(), io.EOF, "LogRoundTrip should report a component status error wrapping its error parameter")
		assert.Equal(t, componentstatus.StatusRecoverableError, event.Status(), "LogRoundTrip on an error parameter should report a recoverable error")
	default:
		require.Fail(t, "LogRoundTrip with an error should report a recoverable error status")
	}

	// Pass in an http error status and make sure it's sent to the component status reporter
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized"},
		nil, time.Now(), 0)
	select {
	case event := <-statusChan:
		err := event.Err()
		require.Error(t, err, "LogRoundTrip with an http error status should report a component status error")
		assert.Contains(t, err.Error(), "401 Unauthorized", "LogRoundTrip with an http error status should include the status in its error state")
		assert.Equal(t, componentstatus.StatusRecoverableError, event.Status(), "LogRoundTrip with an http error status should report a recoverable error")
	default:
		require.Fail(t, "LogRoundTrip with an http error code should report a recoverable error status")
	}

	// Pass in a 409 (duplicate document) and make sure it doesn't report a new status
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusConflict, Status: "409 duplicate"},
		nil, time.Now(), 0)
	select {
	case <-statusChan:
		assert.Fail(t, "LogRoundTrip with a 409 should not change the component status")
	default:
	}

	// Pass in an http success status and make sure the component status returns to OK
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusOK}, nil, time.Now(), 0)
	select {
	case event := <-statusChan:
		assert.NoError(t, event.Err(), "LogRoundTrip with a success status shouldn't report a component status error")
		assert.Equal(t, componentstatus.StatusOK, event.Status(), "LogRoundTrip with a success status should report component status OK")
	default:
		require.Fail(t, "LogRoundTrip with an http success should report component status OK")
	}
}

type testStatusReporter struct {
	statusChan chan *componentstatus.Event
}

func (tsr *testStatusReporter) Report(event *componentstatus.Event) {
	tsr.statusChan <- event
}

func (*testStatusReporter) GetExtensions() map[component.ID]component.Component {
	return make(map[component.ID]component.Component)
}

// Mock transport for testing
type mockEsTransport struct {
	performFunc func(*http.Request) (*http.Response, error)
}

func (m *mockEsTransport) Perform(req *http.Request) (*http.Response, error) {
	return m.performFunc(req)
}

func TestEsClient_Perform_413_Splitting(t *testing.T) {
	lines := []string{
		`{"index":{}}`, `{"field":"value1"}`,
		`{"index":{}}`, `{"field":"value2"}`,
		`{"index":{}}`, `{"field":"value3"}`,
		`{"index":{}}`, `{"field":"value4"}`,
	}
	fullBody := ""
	for _, line := range lines {
		fullBody += line + "\n"
	}

	callCount := 0
	transport := &mockEsTransport{
		performFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)

			if callCount == 1 {
				assert.Equal(t, fullBody, bodyStr)
				return &http.Response{
					StatusCode: http.StatusRequestEntityTooLarge,
					Body:       io.NopCloser(strings.NewReader(`{"error": "too large"}`)),
				}, nil
			}

			assert.Contains(t, fullBody, bodyStr)
			assert.Less(t, len(bodyStr), len(fullBody), "Split body should be smaller than full body")

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
			}, nil
		},
	}

	client := &esClient{transport: transport}

	req, _ := http.NewRequest(http.MethodPost, defaultURL+"/_bulk", strings.NewReader(fullBody))
	resp, err := client.Perform(req)

	// We expect success (200 OK) after splitting.
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Greater(t, callCount, 1, "Expected client to retry request by splitting")
}
