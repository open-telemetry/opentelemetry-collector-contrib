// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type splunkEntClient struct {
	endpoint  *url.URL
	client    *http.Client
	basicAuth string
}

func newSplunkEntClient(cfg *Config) splunkEntClient {
	// tls party
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}

	endpoint, _ := url.Parse(cfg.Endpoint)

	// build and encode our auth string. Do this work once to avoid rebuilding the
	// auth header every time we make a new request
	authString := fmt.Sprintf("%s:%s", cfg.Username, cfg.Password)
	auth64 := base64.StdEncoding.EncodeToString([]byte(authString))
	basicAuth := fmt.Sprintf("Basic %s", auth64)

	return splunkEntClient{
		client:    client,
		endpoint:  endpoint,
		basicAuth: basicAuth,
	}
}

// For running ad hoc searches only
func (c *splunkEntClient) createRequest(sr *searchResponse) (*http.Request, error) {
	// Running searches via Splunk's REST API is a two step process: First you submit the job to run
	// this returns a jobid which is then used in the second part to retrieve the search results
	if sr.Jobid == nil {
		method := "POST"
		path := "/services/search/jobs/"
		url, _ := url.JoinPath(c.endpoint.String(), path)

		// reader for the response data
		data := strings.NewReader(sr.search)

		// return the build request, ready to be run by makeRequest
		req, err := http.NewRequest(method, url, data)
		if err != nil {
			fmt.Println("error building request")
			return nil, err
		}

		// Required headers
		req.Header.Add("Authorization", c.basicAuth)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		return req, nil
	} else {
		method := "GET"
		path := fmt.Sprintf("/services/search/jobs/%s/results", *sr.Jobid)
		url, _ := url.JoinPath(c.endpoint.String(), path)

		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			fmt.Println("error building request")
			return nil, err
		}

		// Required headers
		req.Header.Add("Authorization", c.basicAuth)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		return req, nil
	}
}

func (c *splunkEntClient) createAPIRequest(apiEndpoint string) (*http.Request, error) {
	method := "GET"
	path := fmt.Sprint(apiEndpoint)
	url := c.endpoint.String() + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println("error building request")
		return nil, err
	}

	// Required headers
	req.Header.Add("Authorization", c.basicAuth)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return req, nil
}

// Construct and perform a request to the API. Returns the searchResponse passed into the
// function as state
func (c *splunkEntClient) makeRequest(req *http.Request) (*http.Response, error) {
	res, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("\n error performing request:\n %v", err)
		return nil, err
	}

	return res, nil
}
