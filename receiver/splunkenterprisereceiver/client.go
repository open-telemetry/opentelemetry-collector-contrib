// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/component"
)

// Indexer type "enum". Included in context sent from scraper functions
const (
	typeIdx = "IDX"
	typeSh  = "SH"
	typeCm  = "CM"
)

var (
	errCtxMissingEndpointType = errors.New("context was passed without the endpoint type included")
)

// Type wrapper for accessing context value
type endpointType string

type splunkEntClient struct {
	client    *http.Client
	endpoints map[any]*url.URL
}

func newSplunkEntClient(ctx context.Context, cfg *Config, h component.Host, s component.TelemetrySettings) (*splunkEntClient, error) {
	endpoints := make(map[any]*url.URL)
	client, err := cfg.HTTPClientSettings.ToClient(h, s)
	if err != nil {
		return nil, err
	}

	// if the endpoint is defined, put it in the endpoints map for later use
	// we already checked that url.Parse does not fail in cfg.Validate()
	if cfg.IdxEndpoint != "" {
		endpoints[typeIdx], _ = url.Parse(cfg.IdxEndpoint)
	}
	if cfg.SHEndpoint != "" {
		endpoints[typeSh], _ = url.Parse(cfg.SHEndpoint)
	}
	if cfg.CMEndpoint != "" {
		endpoints[typeCm], _ = url.Parse(cfg.CMEndpoint)
	}

	return &splunkEntClient{
		client:    client,
		endpoints: endpoints,
	}, nil
}

// For running ad hoc searches only
func (c *splunkEntClient) createRequest(ctx context.Context, sr *searchResponse) (*http.Request, error) {
	// get endpoint type from the context
	eptType := ctx.Value(endpointType("type"))
	if eptType == nil {
		return nil, errCtxMissingEndpointType
	}

	// Running searches via Splunk's REST API is a two step process: First you submit the job to run
	// this returns a jobid which is then used in the second part to retrieve the search results
	if sr.Jobid == nil {
		path := "/services/search/jobs/"
		url, _ := url.JoinPath(c.endpoints[eptType].String(), path)

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
	url, _ := url.JoinPath(c.endpoints[eptType].String(), path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *splunkEntClient) createAPIRequest(ctx context.Context, apiEndpoint string) (*http.Request, error) {
	// get endpoint type from the context
	eptType := ctx.Value(endpointType("type"))
	if eptType == nil {
		return nil, errCtxMissingEndpointType
	}

	url := c.endpoints[eptType].String() + apiEndpoint

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
