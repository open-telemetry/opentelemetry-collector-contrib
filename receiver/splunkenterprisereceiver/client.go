// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/component"
)

type splunkEntClient struct {
	client   *http.Client
	endpoint *url.URL
}

func newSplunkEntClient(cfg *Config, h component.Host, s component.TelemetrySettings) (*splunkEntClient, error) {
	client, err := cfg.HTTPClientSettings.ToClient(h, s)
	if err != nil {
		return nil, err
	}

	endpoint, _ := url.Parse(cfg.Endpoint)

	return &splunkEntClient{
		client:   client,
		endpoint: endpoint,
	}, nil
}

// For running ad hoc searches only
func (c *splunkEntClient) createRequest(ctx context.Context, sr *searchResponse) (*http.Request, error) {
	// Running searches via Splunk's REST API is a two step process: First you submit the job to run
	// this returns a jobid which is then used in the second part to retrieve the search results
	if sr.Jobid == nil {
		path := "/services/search/jobs/"
		url, _ := url.JoinPath(c.endpoint.String(), path)

		// reader for the response data
		data := strings.NewReader(sr.search)

		// return the build request, ready to be run by makeRequest
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, data)
		if err != nil {
			return nil, err
		}

		return req, nil
	}
	path := fmt.Sprintf("/services/search/jobs/%s/results", *sr.Jobid)
	url, _ := url.JoinPath(c.endpoint.String(), path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *splunkEntClient) createAPIRequest(ctx context.Context, apiEndpoint string) (*http.Request, error) {
	url := c.endpoint.String() + apiEndpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// Construct and perform a request to the API. Returns the searchResponse passed into the
// function as state
func (c *splunkEntClient) makeRequest(req *http.Request) (*http.Response, error) {
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
