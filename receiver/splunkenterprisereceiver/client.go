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
	errEndpointTypeNotFound   = errors.New("requested client is not configured and could not be found in splunkEntClient")
	errNoClientFound          = errors.New("no client corresponding to the endpoint type was found")
)

// Type wrapper for accessing context value
type endpointType string

// Wrapper around splunkClientMap to avoid awkward reference/dereference stuff that arises when using maps in golang
type splunkEntClient struct {
	clients splunkClientMap
}

// The splunkEntClient is made up of a number of splunkClients defined for each configured endpoint
type splunkClientMap map[string]splunkClient

// The client does not carry the endpoint that is configured with it and golang does not support mixed
// type arrays so this struct contains the pair: the client configured for the endpoint and the endpoint
// itself
type splunkClient struct {
	client   *http.Client
	endpoint *url.URL
}

func newSplunkEntClient(ctx context.Context, cfg *Config, h component.Host, s component.TelemetrySettings) (*splunkEntClient, error) {
	var err error
	var e *url.URL
	var c *http.Client
	clientMap := make(splunkClientMap)

	// if the endpoint is defined, put it in the endpoints map for later use
	// we already checked that url.Parse does not fail in cfg.Validate()
	if cfg.IdxEndpoint.Endpoint != "" {
		e, _ = url.Parse(cfg.IdxEndpoint.Endpoint)
		c, err = cfg.IdxEndpoint.ToClient(ctx, h, s)
		if err != nil {
			return nil, err
		}
		clientMap[typeIdx] = splunkClient{
			client:   c,
			endpoint: e,
		}
	}
	if cfg.SHEndpoint.Endpoint != "" {
		e, _ = url.Parse(cfg.SHEndpoint.Endpoint)
		c, err = cfg.SHEndpoint.ToClient(ctx, h, s)
		if err != nil {
			return nil, err
		}
		clientMap[typeSh] = splunkClient{
			client:   c,
			endpoint: e,
		}
	}
	if cfg.CMEndpoint.Endpoint != "" {
		e, _ = url.Parse(cfg.CMEndpoint.Endpoint)
		c, err = cfg.CMEndpoint.ToClient(ctx, h, s)
		if err != nil {
			return nil, err
		}
		clientMap[typeCm] = splunkClient{
			client:   c,
			endpoint: e,
		}
	}

	return &splunkEntClient{clients: clientMap}, nil
}

// For running ad hoc searches only
func (c *splunkEntClient) createRequest(eptType string, sr *searchResponse) (req *http.Request, err error) {
	ctx := context.WithValue(context.Background(), endpointType("type"), eptType)

	// Running searches via Splunk's REST API is a two step process: First you submit the job to run
	// this returns a jobid which is then used in the second part to retrieve the search results
	if sr.Jobid == nil {
		var u string
		path := "/services/search/jobs/"

		if e, ok := c.clients[eptType]; ok {
			u, err = url.JoinPath(e.endpoint.String(), path)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errNoClientFound
		}

		// reader for the response data
		data := strings.NewReader(sr.search)

		// return the build request, ready to be run by makeRequest
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, u, data)
		if err != nil {
			return nil, err
		}

		return req, nil
	}
	path := fmt.Sprintf("/services/search/jobs/%s/results", *sr.Jobid)
	url, _ := url.JoinPath(c.clients[eptType].endpoint.String(), path)

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// forms an *http.Request for use with Splunk built-in API's (like introspection).
func (c *splunkEntClient) createAPIRequest(eptType string, apiEndpoint string) (req *http.Request, err error) {
	var u string
	ctx := context.WithValue(context.Background(), endpointType("type"), eptType)

	if e, ok := c.clients[eptType]; ok {
		u = e.endpoint.String() + apiEndpoint
	} else {
		return nil, errNoClientFound
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// Perform a request.
func (c *splunkEntClient) makeRequest(req *http.Request) (*http.Response, error) {
	// get endpoint type from the context
	eptType := req.Context().Value(endpointType("type"))
	if eptType == nil {
		return nil, errCtxMissingEndpointType
	}

	var endpointType string
	switch t := eptType.(type) {
	case string:
		endpointType = t
	default:
		endpointType = fmt.Sprintf("%v", eptType)
	}

	if sc, ok := c.clients[endpointType]; ok {
		res, err := sc.client.Do(req)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return nil, errEndpointTypeNotFound
}

// Check if the splunkEntClient contains a configured endpoint for the type of scraper
// Returns true if an entry exists, false if not.
func (c *splunkEntClient) isConfigured(v string) bool {
	_, ok := c.clients[v]
	return ok
}
