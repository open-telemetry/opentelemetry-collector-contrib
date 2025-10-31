// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfish // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/redfish"

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

type Client struct {
	client           http.Client
	BaseURL          *url.URL
	redfishVersion   string
	host             string
	userName         string
	password         configopaque.String
	computerSystemID string
}

// redfish client options
type ClientOption func(*clientOptions)

type clientOptions struct {
	RedfishVersion string
	ClientTimeout  time.Duration
	Insecure       bool
}

func WithRedfishVersion(version string) ClientOption {
	return func(o *clientOptions) { o.RedfishVersion = version }
}

func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.ClientTimeout = timeout }
}

func WithInsecure(insecure bool) ClientOption {
	return func(o *clientOptions) { o.Insecure = insecure }
}

// NewRedfishClient is a function to create new redfish clients
func NewClient(computerSystemID, addr, user string, pwd configopaque.String, opts ...ClientOption) (*Client, error) {
	baseURL, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	address, _, err := net.SplitHostPort(baseURL.Host)
	if err != nil {
		address = baseURL.Host
	}

	clientOpts := &clientOptions{
		RedfishVersion: "v1",
		ClientTimeout:  time.Second * 60,
		Insecure:       false,
	}
	for _, opt := range opts {
		opt(clientOpts)
	}

	return &Client{
		client: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: clientOpts.Insecure},
				Proxy:           http.ProxyFromEnvironment,
			},
			Timeout: clientOpts.ClientTimeout,
		},
		computerSystemID: computerSystemID,
		BaseURL:          baseURL,
		host:             address,
		redfishVersion:   clientOpts.RedfishVersion,
		userName:         user,
		password:         pwd,
	}, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.SetBasicAuth(c.userName, string(c.password))
	req.Header.Set("OData-Version", "4.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
}

// GetComputerSystem is a method to get the client's computer system data
func (c *Client) GetComputerSystem() (*ComputerSystem, error) {
	url := c.BaseURL.ResolveReference(&url.URL{
		Path: path.Join("/redfish/", c.redfishVersion, "/Systems/", c.computerSystemID),
	}).String()
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var system ComputerSystem
	err = json.NewDecoder(resp.Body).Decode(&system)
	if err != nil {
		return nil, err
	}
	return &system, nil
}

// GetChassis is a method that gets chassis data given a chassis odata ref
func (c *Client) GetChassis(ref string) (*Chassis, error) {
	url := c.BaseURL.ResolveReference(&url.URL{Path: ref}).String()
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var chassis Chassis
	err = json.NewDecoder(resp.Body).Decode(&chassis)
	if err != nil {
		return nil, err
	}
	return &chassis, nil
}

// GetThermal is a method that gets thermal data given a thermal odata ref
func (c *Client) GetThermal(ref string) (*Thermal, error) {
	url := c.BaseURL.ResolveReference(&url.URL{Path: ref}).String()
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var thermal Thermal
	err = json.NewDecoder(resp.Body).Decode(&thermal)
	if err != nil {
		return nil, err
	}
	return &thermal, nil
}
