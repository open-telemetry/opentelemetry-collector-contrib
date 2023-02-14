// Copyright The OpenTelemetry Authors
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

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"
)

func TestNewClient(t *testing.T) {
	testCase := []struct {
		desc        string
		cfg         *Config
		expectError error
	}{
		{
			desc: "Valid Configuration with key and email auth",
			cfg: &Config{
				PollInterval: defaultPollInterval,
				Zone:         "1",
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectError: nil,
		},
		{
			desc: "Valid Configuration with api token auth",
			cfg: &Config{
				PollInterval: defaultPollInterval,
				Zone:         "1",
				Auth: &Auth{
					APIToken: "abc123",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newCloudflareClient(tc.cfg, defaultBaseURL)
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.Contains(t, err.Error(), tc.expectError.Error())
			} else {
				require.NoError(t, err)
				actualClient, ok := ac.(*cloudflareClient)
				require.True(t, ok)
				require.NotNil(t, actualClient.api)
			}
		})
	}
}

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// client is the API client being tested.
	cc client

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server

	startTime = "2023-01-17T22:18:01-05:00"
	endTime   = "2023-01-17T22:18:11-05:00"
)

func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// Cloudflare client configured to use test server
	cc, _ = newCloudflareClient(&Config{
		PollInterval: defaultPollInterval,
		Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
		Auth: &Auth{
			XAuthKey:   "abc123",
			XAuthEmail: "email@email.com",
		},
		Logs: &LogsConfig{
			Sample: float32(defaultSampleRate),
			Count:  defaultCount,
			Fields: defaultFields,
		},
	}, server.URL)
}

func teardown() {
	server.Close()
}

func TestMakeRequest(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Successful call",
			testFunc: func(*testing.T) {
				setup()
				defer teardown()
				handler := func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
					w.Header().Set("content-type", "application/json")
					fmt.Fprintf(w, `{
						"errors": [],
						"messages": [],
						"result": [
							{
								"ClientIP": "89.163.253.200",
								"ClientRequestHost": "www.theburritobot0.com",
								"ClientRequestMethod": "GET",
								"ClientRequestURI": "/static/img/testimonial-hipster.png",
								"EdgeEndTimestamp": 1506702504461999900,
								"EdgeResponseBytes": 69045,
								"EdgeResponseStatus": 200,
								"EdgeStartTimestamp": 1506702504433000200,
								"RayID": "3a6050bcbe121a87"
							},
							{
								"ClientIP": "89.163.253.201",
								"ClientRequestHost": "www.theburritobot1.com",
								"ClientRequestMethod": "GET",
								"ClientRequestURI": "/static/img/testimonial-hipster.png",
								"EdgeEndTimestamp": 1506702504461999900,
								"EdgeResponseBytes": 69045,
								"EdgeResponseStatus": 200,
								"EdgeStartTimestamp": 1506702504433000200,
								"RayID": "3a6050bcbe121a87"
							}
						],
						"success": true
					}`)
				}

				pattern := "/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received"
				mux.HandleFunc(pattern, handler)
				want := []*models.Log{
					{
						ClientIP:            &[]string{"89.163.253.200"}[0],
						ClientRequestHost:   &[]string{"www.theburritobot0.com"}[0],
						ClientRequestMethod: &[]string{"GET"}[0],
						ClientRequestURI:    &[]string{"/static/img/testimonial-hipster.png"}[0],
						EdgeEndTimestamp:    &[]int64{1506702504461999900}[0],
						EdgeResponseBytes:   &[]int64{69045}[0],
						EdgeResponseStatus:  &[]int64{200}[0],
						EdgeStartTimestamp:  &[]int64{1506702504433000200}[0],
						RayID:               &[]string{"3a6050bcbe121a87"}[0],
					},
					{
						ClientIP:            &[]string{"89.163.253.201"}[0],
						ClientRequestHost:   &[]string{"www.theburritobot1.com"}[0],
						ClientRequestMethod: &[]string{"GET"}[0],
						ClientRequestURI:    &[]string{"/static/img/testimonial-hipster.png"}[0],
						EdgeEndTimestamp:    &[]int64{1506702504461999900}[0],
						EdgeResponseBytes:   &[]int64{69045}[0],
						EdgeResponseStatus:  &[]int64{200}[0],
						EdgeStartTimestamp:  &[]int64{1506702504433000200}[0],
						RayID:               &[]string{"3a6050bcbe121a87"}[0],
					},
				}

				actual, err := cc.MakeRequest(context.Background(), startTime, endTime)
				require.NoError(t, err)
				require.Equal(t, want, actual)
			},
		},
		{
			desc: "Unauthorized response",
			testFunc: func(*testing.T) {
				setup()
				defer teardown()
				handler := func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
					w.Header().Set("content-type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
				}

				pattern := "/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received"
				mux.HandleFunc(pattern, handler)
				expectedError := errors.New("failed to retrieve Cloudflare logs: error unmarshalling the JSON response error body: unexpected end of JSON input")

				_, err := cc.MakeRequest(context.Background(), startTime, endTime)
				require.Error(t, err)
				require.ErrorContains(t, expectedError, err.Error())
			},
		},
		{
			desc: "Bad response",
			testFunc: func(*testing.T) {
				setup()
				defer teardown()

				handler := func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
					w.Header().Set("content-type", "application/json")
					_, err := w.Write([]byte("{}"))
					require.NoError(t, err)
				}

				pattern := "/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received"
				mux.HandleFunc(pattern, handler)
				expectedError := errors.New("failed to unmarshal response body: unexpected end of JSON input")

				_, err := cc.MakeRequest(context.Background(), startTime, endTime)
				require.Error(t, err)
				require.ErrorContains(t, expectedError, err.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestMakeRequestBadResponse(t *testing.T) {
	setup()
	defer teardown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
	}

	pattern := "/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received"
	mux.HandleFunc(pattern, handler)
	expectedError := errors.New("failed to retrieve Cloudflare logs: error unmarshalling the JSON response error body: unexpected end of JSON input")

	_, err := cc.MakeRequest(context.Background(), startTime, endTime)
	require.Error(t, err)
	require.ErrorContains(t, expectedError, err.Error())
}

func TestMakeRequestBadPayload(t *testing.T) {
	setup()
	defer teardown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("content-type", "application/json")
		_, err := w.Write([]byte("{}"))
		require.NoError(t, err)
	}

	pattern := "/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received"
	mux.HandleFunc(pattern, handler)
	expectedError := errors.New("failed to unmarshal response body: unexpected end of JSON input")

	_, err := cc.MakeRequest(context.Background(), startTime, endTime)
	require.Error(t, err)
	require.ErrorContains(t, expectedError, err.Error())
}

func TestBuildEndpoint(t *testing.T) {
	zone := "123abc"
	logsFields := "fields=ClientIP,ClientRequestHost,EdgeEndTimestamp,EdgeResponseStatus&sample=0.550000&count=1"

	cfg := &Config{
		PollInterval: defaultPollInterval,
		Zone:         zone,
		Auth: &Auth{
			XAuthKey:   "abc123",
			XAuthEmail: "email@email.com",
		},
		Logs: &LogsConfig{
			Sample: 0.55,
			Count:  1,
			Fields: []string{"ClientIP", "ClientRequestHost", "EdgeEndTimestamp", "EdgeResponseStatus"},
		},
	}
	cloudflareClient, err := newCloudflareClient(cfg, defaultBaseURL)
	require.NoError(t, err)
	endpoint := cloudflareClient.BuildEndpoint(startTime, endTime)

	expectedEndpoint := fmt.Sprintf("/zones/%s/logs/received?start=%s&end=%s&%s", zone, startTime, endTime, logsFields)
	require.Equal(t, expectedEndpoint, endpoint)
}

var (
	multiLogs   = "./testdata/example-logs/multi-logs.json"
	singleLog   = "./testdata/example-logs/single-log.json"
	partialLogs = "./testdata/example-logs/partial-log.json"
)

func loadTestFile(filePath string) ([]*models.Log, error) {
	var logs []*models.Log
	testFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(testFile, &logs)
	if err != nil {
		return nil, err
	}

	return logs, nil
}
