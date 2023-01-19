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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"
	"github.com/stretchr/testify/require"
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
		{
			desc: "Invalid Configuration, bad auth",
			cfg: &Config{
				PollInterval: defaultPollInterval,
				Zone:         "1",
				Auth:         &Auth{},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectError: errInvalidAuthenticationConfigured,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newCloudflareClient(tc.cfg)
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

func setup(opts ...cloudflare.Option) {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// disable rate limits and retries in testing - prepended so any provided value overrides this
	opts = append([]cloudflare.Option{cloudflare.UsingRateLimit(100000), cloudflare.UsingRetryPolicy(0, 0, 0)}, opts...)

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
	})
	cc.SetEndpoint(server.URL)
}

func teardown() {
	server.Close()
}

// func TestMakeRequest(t *testing.T) {
// 	setup()
// 	defer teardown()

// 	handler := func(w http.ResponseWriter, r *http.Request) {
// 		require.Equal(t, http.MethodGet, r.Method, "Expected method 'GET', got %s", r.Method)
// 		w.Header().Set("content-type", "application/json")
// 		fmt.Fprintf(w, `{
// 		"ClientIP": "89.163.242.206",
// 		"ClientRequestHost": "www.theburritobot.com",
// 		"ClientRequestMethod": "GET",
// 		"ClientRequestURI": "/static/img/testimonial-hipster.png",
// 		"EdgeEndTimestamp": 1506702504461999900,
// 		"EdgeResponseBytes": 69045,
// 		"EdgeResponseStatus": 200,
// 		"EdgeStartTimestamp": 1506702504433000200,
// 		"RayID": "3a6050bcbe121a87"
// 	}`)
// 	}

// 	// pattern := "http://127.0.0.1:51068/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received?start=2023-01-17T22:18:01-05:00&end=2023-01-17T22:18:11-05:00&fields=ClientIP,ClientRequestHost,ClientRequestMethod,ClientRequestURI,EdgeEndTimestamp,EdgeResponseBytes,EdgeResponseStatus,EdgeStartTimestamp,RayID&sample=1.000000"
// 	// pattern := "http://127.0.0.1:51068/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received?start=2023-01-17T22:18:01-05:00&end=2023-01-17T22:18:11-05:00&fields=ClientIP,ClientRequestHost,ClientRequestMethod,ClientRequestURI,EdgeEndTimestamp,EdgeResponseBytes,EdgeResponseStatus,EdgeStartTimestamp,RayID&sample=1.000000"
// 	// mux.HandleFunc(pattern, handler)
// 	mux.HandleFunc("/zones/023e105f4ecef8ad9ca31a8372d0c353/logs/received", handler)
// 	want := []*models.Log{
// 		{
// 			ClientIP:            "89.163.242.206",
// 			ClientRequestHost:   "www.theburritobot.com",
// 			ClientRequestMethod: "GET",
// 			ClientRequestURI:    "/static/img/testimonial-hipster.png",
// 			EdgeEndTimestamp:    1506702504461999900,
// 			EdgeResponseBytes:   69045,
// 			EdgeResponseStatus:  200,
// 			EdgeStartTimestamp:  1506702504433000200,
// 			RayID:               "3a6050bcbe121a87",
// 		},
// 	}

// 	actual, err := cc.MakeRequest(context.Background(), server.URL, "2023-01-17T22:18:01-05:00", "2023-01-17T22:18:11-05:00")
// 	require.NoError(t, err)
// 	require.Equal(t, want, actual)
// }

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
	client, err := newCloudflareClient(cfg)
	require.NoError(t, err)
	endpoint := client.BuildEndpoint(defaultBaseURL, startTime, endTime)

	expectedEndpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/logs/received?start=%s&end=%s&%s", zone, startTime, endTime, logsFields)
	require.Equal(t, expectedEndpoint, endpoint)
}

var (
	multiLogs = "./testdata/example-logs/multi-logs.json"
	singleLog = "./testdata/example-logs/single-log.json"
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
