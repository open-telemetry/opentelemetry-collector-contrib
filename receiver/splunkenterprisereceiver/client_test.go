// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestClientCreation(t *testing.T) {
	// create a client from an example config
	client := newSplunkEntClient(&Config{
		Username:          "admin",
		Password:          "securityFirst",
		MaxSearchWaitTime: 11 * time.Second,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
	})

	testEndpoint, _ := url.Parse("https://localhost:8089")

	authString := fmt.Sprintf("%s:%s", "admin", "securityFirst")
	auth64 := base64.StdEncoding.EncodeToString([]byte(authString))
	testBasicAuth := fmt.Sprintf("Basic %s", auth64)

	require.Equal(t, client.endpoint, testEndpoint)
	require.Equal(t, client.basicAuth, testBasicAuth)
}

// test functionality of createRequest which is used for building metrics out of
// ad-hoc searches
func TestClientCreateRequest(t *testing.T) {
	// create a client from an example config
	client := newSplunkEntClient(&Config{
		Username:          "admin",
		Password:          "securityFirst",
		MaxSearchWaitTime: 11 * time.Second,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
	})

	testJobID := "123"

	tests := []struct {
		desc     string
		sr       *searchResponse
		client   splunkEntClient
		expected *http.Request
	}{
		{
			desc: "First req, no jobid",
			sr: &searchResponse{
				search: "example search",
			},
			client: client,
			expected: func() *http.Request {
				method := "POST"
				path := "/services/search/jobs/"
				testEndpoint, _ := url.Parse("https://localhost:8089")
				url, _ := url.JoinPath(testEndpoint.String(), path)
				data := strings.NewReader("example search")
				req, _ := http.NewRequest(method, url, data)
				req.Header.Add("Authorization", client.basicAuth)
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				return req
			}(),
		},
		{
			desc: "Second req, jobID detected",
			sr: &searchResponse{
				search: "example search",
				Jobid:  &testJobID,
			},
			client: client,
			expected: func() *http.Request {
				method := "GET"
				path := fmt.Sprintf("/services/search/jobs/%s/results", testJobID)
				testEndpoint, _ := url.Parse("https://localhost:8089")
				url, _ := url.JoinPath(testEndpoint.String(), path)
				req, _ := http.NewRequest(method, url, nil)
				req.Header.Add("Authorization", client.basicAuth)
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				return req
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			req, err := test.client.createRequest(test.sr)
			require.NoError(t, err)
			// have to test specific parts since individual fields are pointers
			require.Equal(t, test.expected.URL, req.URL)
			require.Equal(t, test.expected.Method, req.Method)
			require.Equal(t, test.expected.Header, req.Header)
			require.Equal(t, test.expected.Body, req.Body)
		})
	}
}

// createAPIRequest creates a request for api calls i.e. to introspection endpoint
func TestAPIRequestCreate(t *testing.T) {
	client := newSplunkEntClient(&Config{
		Username:          "admin",
		Password:          "securityFirst",
		MaxSearchWaitTime: 11 * time.Second,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
	})

	req, err := client.createAPIRequest("/test/endpoint")
	require.NoError(t, err)

	expectedUrl := client.endpoint.String() + "/test/endpoint"
	expected, _ := http.NewRequest(http.MethodGet, expectedUrl, nil)
	expected.Header.Add("Authorization", client.basicAuth)
	expected.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	require.Equal(t, expected.URL, req.URL)
	require.Equal(t, expected.Method, req.Method)
	require.Equal(t, expected.Header, req.Header)
	require.Equal(t, expected.Body, req.Body)
}
