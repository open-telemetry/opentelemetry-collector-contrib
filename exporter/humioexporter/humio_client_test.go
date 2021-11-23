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

package humioexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
)

func makeClient(t *testing.T, host string, compression bool) exporterClient {
	cfg := &Config{
		ExporterSettings:   config.NewExporterSettings(config.NewComponentID(typeStr)),
		DisableCompression: !compression,
		Tag:                TagNone,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: host,
		},
		Logs: LogsConfig{
			IngestToken: "logs-token",
		},
		Traces: TracesConfig{
			IngestToken: "traces-token",
		},
	}
	err := cfg.Validate()
	require.NoError(t, err)

	err = cfg.sanitize()
	require.NoError(t, err)

	client, err := newHumioClient(cfg, zap.NewNop(), componenttest.NewNopHost())
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
	loc, _ := time.LoadLocation("Europe/Copenhagen")
	timestamp := time.Date(2021, 3, 28, 12, 30, 15, 0, loc)

	return []*HumioStructuredEvents{
		// Fully specified
		{
			Tags: map[string]string{
				"tag1": "tagval1",
				"tag2": "tagval2",
			},
			Events: []*HumioStructuredEvent{
				{
					Timestamp: timestamp,
					AsUnix:    unix,
					Attributes: map[string]string{
						"attr1": "attrval1",
						"attr2": "attrval2",
					},
				},
			},
		},
		// Only required fields
		{
			Events: []*HumioStructuredEvent{
				{
					Timestamp: timestamp,
					AsUnix:    unix,
				},
				{
					Timestamp: timestamp,
					AsUnix:    unix,
				},
			},
		},
	}
}

type requestData struct {
	Path   string
	Header http.Header
	Body   string
	Error  error
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
		result.Header = r.Header
		body, err := ioutil.ReadAll(r.Body)

		if err != nil {
			result.Error = err
		} else {
			result.Body = string(body)
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
	c := makeClient(t, "http://localhost:8080", true)

	// Assert
	assert.NotNil(t, c)
}

func TestSendUnstructuredEvents(t *testing.T) {
	// Arrange
	expected := `[{"fields":{"field1":"fieldval1"},"tags":{"tag1":"tagval1","tag2":"tagval2"},"type":"custom-parser","messages":["msg1","msg2","msg3"]},{"messages":["msg1","msg2"]}]`
	evts := makeUnstructuredEvents()

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL, false)
		return humio.sendUnstructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Contains(t, result.Header.Get("authorization"), "Bearer logs-token")
	assert.Equal(t, "/api/v1/ingest/humio-unstructured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendStructuredEventsIso(t *testing.T) {
	// Arrange
	expected := `[{"tags":{"tag1":"tagval1","tag2":"tagval2"},"events":[{"timestamp":"2021-03-28T12:30:15+02:00","attributes":{"attr1":"attrval1","attr2":"attrval2"}}]},{"events":[{"timestamp":"2021-03-28T12:30:15+02:00"},{"timestamp":"2021-03-28T12:30:15+02:00"}]}]`
	evts := makeStructuredEvents(false)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL, false)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Contains(t, result.Header.Get("authorization"), "Bearer traces-token")
	assert.Equal(t, "/api/v1/ingest/humio-structured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendStructuredEventsUnix(t *testing.T) {
	// Arrange
	expected := `[{"tags":{"tag1":"tagval1","tag2":"tagval2"},"events":[{"timestamp":1616927415000,"timezone":"Europe/Copenhagen","attributes":{"attr1":"attrval1","attr2":"attrval2"}}]},{"events":[{"timestamp":1616927415000,"timezone":"Europe/Copenhagen"},{"timestamp":1616927415000,"timezone":"Europe/Copenhagen"}]}]`
	evts := makeStructuredEvents(true)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL, false)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Contains(t, result.Header.Get("authorization"), "Bearer traces-token")
	assert.Equal(t, "/api/v1/ingest/humio-structured", result.Path)
	assert.Equal(t, expected, result.Body)
}

func TestSendEventsUncompressedHeaders(t *testing.T) {
	// Arrange
	evts := makeStructuredEvents(true)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL, false)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Contains(t, result.Header.Get("authorization"), "Bearer traces-token")
	assert.Contains(t, result.Header.Get("content-type"), "application/json")
	assert.NotEmpty(t, result.Header.Get("user-agent"))
	assert.Empty(t, result.Header.Get("content-encoding"))
}

func TestSendEventsCompressed(t *testing.T) {
	// Arrange
	evts := makeStructuredEvents(true)
	payload, err := json.Marshal(evts)
	require.NoError(t, err)

	expected := new(bytes.Buffer)
	writer := gzip.NewWriter(expected)
	_, err = writer.Write(payload)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	// Act
	result := executeRequest(func(s *httptest.Server) error {
		humio := makeClient(t, s.URL, true)
		return humio.sendStructuredEvents(context.Background(), evts)
	})

	// Assert
	require.NoError(t, result.Error)
	assert.Contains(t, result.Header.Get("content-encoding"), "gzip")
	assert.Equal(t, "/api/v1/ingest/humio-structured", result.Path)
	assert.Equal(t, expected.String(), result.Body)
}

func TestSendEventsNoConnection(t *testing.T) {
	// Arrange
	humio := makeClient(t, "https://localhost:8080", true)

	// Act
	err := humio.sendStructuredEvents(context.Background(), makeStructuredEvents(false))

	// Assert
	require.Error(t, err)
	assert.False(t, consumererror.IsPermanent(err))
}

func TestSendEventsBadParameters(t *testing.T) {
	// Arrange
	humio := makeClient(t, "https://localhost:8080", true)

	// Act
	err := humio.(*humioClient).sendEvents(context.Background(), nil, "\n", "token")

	// Assert
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

type problematicStruct struct{}

func (e problematicStruct) MarshalJSON() ([]byte, error) {
	return nil, errors.New("Fail")
}

func TestSendStructuredEventsMarshalError(t *testing.T) {
	// Arrange
	humio := makeClient(t, "https://localhost:8080", true)
	evts := []*HumioStructuredEvents{
		{
			Events: []*HumioStructuredEvent{
				{
					Timestamp:  time.Now(),
					Attributes: problematicStruct{},
				},
			},
		},
	}

	// Act
	err := humio.sendStructuredEvents(context.Background(), evts)

	// Assert
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

func TestSendEventsStatusCodes(t *testing.T) {
	// Arrange
	testCases := []struct {
		desc     string
		code     int
		wantPerm bool
	}{
		{
			desc:     "Retry on Not Found",
			code:     404,
			wantPerm: false,
		},
		{
			desc:     "Retry on Request Timeout",
			code:     404,
			wantPerm: false,
		},
		{
			desc:     "Retry on Internal Server Error",
			code:     500,
			wantPerm: false,
		},
		{
			desc:     "Retry on Service Unavailable",
			code:     503,
			wantPerm: false,
		},
		{
			desc:     "Fail on Bad Request",
			code:     400,
			wantPerm: true,
		},
		{
			desc:     "Fail on Unauthorized",
			code:     401,
			wantPerm: true,
		},
		{
			desc:     "Fail on Forbidden",
			code:     403,
			wantPerm: true,
		},
	}

	// Act
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(tC.code)
			}))
			defer s.Close()

			humio := makeClient(t, s.URL, true)
			err := humio.sendUnstructuredEvents(context.Background(), makeUnstructuredEvents())

			// Assert
			if consumererror.IsPermanent(err) != tC.wantPerm {
				t.Errorf("sendEvents() permanent = %v, wantPerm %v",
					consumererror.IsPermanent(err), tC.wantPerm)
			}
		})
	}
}
