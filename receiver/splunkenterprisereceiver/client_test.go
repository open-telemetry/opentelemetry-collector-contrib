// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// mockHost allows us to create a test host with a no op extension that can be used to satisfy the SDK without having to parse from an
// actual config.yaml.
type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

func TestClientCreation(t *testing.T) {
	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
			Auth: &configauth.Authentication{
				AuthenticatorID: component.NewID("basicauth/client"),
			},
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
			Timeout:            11 * time.Second,
		},
	}

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.NewID("basicauth/client"): auth.NewClient(),
		},
	}
	// create a client from an example config
	client, err := newSplunkEntClient(cfg, host, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	testEndpoint, _ := url.Parse("https://localhost:8089")

	require.Equal(t, client.endpoint, testEndpoint)
}

// test functionality of createRequest which is used for building metrics out of
// ad-hoc searches
func TestClientCreateRequest(t *testing.T) {
	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
			Auth: &configauth.Authentication{
				AuthenticatorID: component.NewID("basicauth/client"),
			},
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
			Timeout:            11 * time.Second,
		},
	}

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.NewID("basicauth/client"): auth.NewClient(),
		},
	}
	// create a client from an example config
	client, err := newSplunkEntClient(cfg, host, componenttest.NewNopTelemetrySettings())

	require.NoError(t, err)

	testJobID := "123"

	tests := []struct {
		desc     string
		sr       *searchResponse
		client   *splunkEntClient
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
				return req
			}(),
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			req, err := test.client.createRequest(ctx, test.sr)
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
	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
			Auth: &configauth.Authentication{
				AuthenticatorID: component.NewID("basicauth/client"),
			},
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
			Timeout:            11 * time.Second,
		},
	}

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.NewID("basicauth/client"): auth.NewClient(),
		},
	}
	// create a client from an example config
	client, err := newSplunkEntClient(cfg, host, componenttest.NewNopTelemetrySettings())

	require.NoError(t, err)

	ctx := context.Background()
	req, err := client.createAPIRequest(ctx, "/test/endpoint")
	require.NoError(t, err)

	// build the expected request
	expectedURL := client.endpoint.String() + "/test/endpoint"
	expected, _ := http.NewRequest(http.MethodGet, expectedURL, nil)

	require.Equal(t, expected.URL, req.URL)
	require.Equal(t, expected.Method, req.Method)
	require.Equal(t, expected.Header, req.Header)
	require.Equal(t, expected.Body, req.Body)
}
