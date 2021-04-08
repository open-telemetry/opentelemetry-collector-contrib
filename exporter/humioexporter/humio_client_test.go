// Copyright 2021, OpenTelemetry Authors
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

package humioexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
)

func makeClient(t *testing.T, host string) *humioClient {
	cfg := &Config{
		IngestToken: "token",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: host,
		},
	}
	err := cfg.sanitize()
	require.NoError(t, err)

	client, err := newHumioClient(cfg, zap.NewNop())
	require.NoError(t, err)
	return client
}

func makeUnstructuredEvents() []*HumioUnstructuredEvents {
	return []*HumioUnstructuredEvents{
		// Fully specified
		{
			Fields: map[string]string{
				"field1": "fieldval1",
			},
			Tags: map[string]string{
				"tag1": "tagval1",
				"tag2": "tagval2",
			},
			Type: "custom-parser",
			Messages: []string{
				"msg1",
				"msg2",
				"msg3",
			},
		},
		// Only required fields
		{
			Messages: []string{
				"msg1",
				"msg2",
			},
		},
	}
}

func makeStructuredEvents(unix bool) []*HumioStructuredEvents {
	var timestamp = time.Date(2021, 3, 28, 12, 30, 15, 0, time.UTC)
	var timeZone = ""
	if unix {
		timeZone = "Europe/Copenhagen"
	}

	return []*HumioStructuredEvents{
		// Fully specified
		{
			Tags: map[string]string{
				"tag1": "tagval1",
				"tag2": "tagval2",
			},
			Events: []*HumioStructuredEvent{
				{
					TimeStamp: timestamp,
					TimeZone:  timeZone,
					Attributes: map[string]string{
						"attr1": "attrval1",
						"attr2": "attrval2",
					},
					RawString: "str1",
				},
			},
		},
		// Only required fields
		{
			Events: []*HumioStructuredEvent{
				{
					TimeStamp: timestamp,
					TimeZone:  timeZone,
				},
				{
					TimeStamp: timestamp,
					TimeZone:  timeZone,
				},
			},
		},
	}
}

type requestData struct {
	Path  string
	Body  string
	Error error
}

// Helper function to intercept information from HTTP requests.
// The caller provides a closure from which it is possible to access the address
// of the mock server. The HTTP request must also be performed from within this
// closure.
func executeRequest(fn func(s *httptest.Server) error) (result requestData) {
	// Create a mock server that will intercept information from the request and
	// store it in "result"
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		result.Path = r.URL.Path
		body, err := io.ReadAll(r.Body)

		if err != nil {
			result.Error = err
		} else {
			result.Body = string(body)
			defer r.Body.Close()
		}
	}))
	defer s.Close()

	// Call the closure to execute the request
	err := fn(s)
	if err != nil {
		result.Error = err
	}

	return result
}

func TestNewHumioClient(t *testing.T) {
	// Arrange / Act
	humio := makeClient(t, "http://localhost:8080")

	// Assert
	assert.NotNil(t, humio)
	assert.NotNil(t, humio.client)
}

func TestSendUnstructuredEvents(t *testing.T) {
	// Arrange
	expected := `[{"fields":{"field1":"fieldval1"},"tags":{"tag1":"tagval1","tag2":"tagval2"},"type":"custom-parser","messages":["msg1","msg2","msg3"]},{"messages":["msg1","msg2"]}]`
	evts := makeUnstructuredEvents()

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL)
		return humio.sendUnstructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Equal(t, "/api/v1/ingest/humio-unstructured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendStructuredEventsIso(t *testing.T) {
	// Arrange
	expected := `[{"tags":{"tag1":"tagval1","tag2":"tagval2"},"events":[{"timestamp":"2021-03-28T12:30:15Z","attributes":{"attr1":"attrval1","attr2":"attrval2"},"rawstring":"str1"}]},{"events":[{"timestamp":"2021-03-28T12:30:15Z"},{"timestamp":"2021-03-28T12:30:15Z"}]}]`
	evts := makeStructuredEvents(false)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Equal(t, "/api/v1/ingest/humio-structured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendStructuredEventsUnix(t *testing.T) {
	// Arrange
	expected := `[{"tags":{"tag1":"tagval1","tag2":"tagval2"},"events":[{"timestamp":"1616927415","timezone":"Europe/Copenhagen","attributes":{"attr1":"attrval1","attr2":"attrval2"},"rawstring":"str1"}]},{"events":[{"timestamp":"1616927415","timezone":"Europe/Copenhagen"},{"timestamp":"1616927415","timezone":"Europe/Copenhagen"}]}]`
	evts := makeStructuredEvents(true)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Equal(t, "/api/v1/ingest/humio-structured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendEventsNoConnection(t *testing.T) {
	// Arrange
	humio := makeClient(t, "https://localhost:8080")

	// Act
	err := humio.sendStructuredEvents(context.Background(), makeStructuredEvents(false))

	// Assert
	require.Error(t, err)
	assert.False(t, consumererror.IsPermanent(err))
}

func TestSendEventsBadParameters(t *testing.T) {
	// Arrange
	humio := makeClient(t, "https://localhost:8080")

	// Act
	err := humio.sendStructuredEvents(nil, nil)

	// Assert
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

// TODO: Test JSON marshal error?
// TODO: Test error codes with test cases
