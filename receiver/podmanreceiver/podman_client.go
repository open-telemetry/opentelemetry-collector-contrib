// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

type clientFactory func(logger *zap.Logger, cfg *Config) (PodmanClient, error)

type PodmanClient interface {
	ping(context.Context) error
	stats(context.Context, url.Values) ([]containerStats, error)
	list(context.Context, url.Values) ([]Container, error)
	events(context.Context, url.Values) (<-chan Event, <-chan error)
}

type libpodClient struct {
	conn     *http.Client
	endpoint string
}

func newLibpodClient(logger *zap.Logger, cfg *Config) (PodmanClient, error) {
	connection, err := newPodmanConnection(logger, cfg.Endpoint, cfg.SSHKey, cfg.SSHPassphrase)
	if err != nil {
		return nil, err
	}
	c := &libpodClient{
		conn:     connection,
		endpoint: fmt.Sprintf("http://d/v%s/libpod", cfg.APIVersion),
	}
	return c, nil
}

func (c *libpodClient) request(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}

	return c.conn.Do(req)
}

func (c *libpodClient) stats(ctx context.Context, options url.Values) ([]containerStats, error) {
	resp, err := c.request(ctx, "/containers/stats", options)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	report := &containerStatsReport{}
	err = json.Unmarshal(bytes, report)
	if err != nil {
		return nil, err
	}
	if report.Error != "" {
		return nil, errors.New(report.Error)
	}
	return report.Stats, nil
}

func (c *libpodClient) list(ctx context.Context, options url.Values) ([]Container, error) {
	resp, err := c.request(ctx, "/containers/json", options)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var report []Container
	err = json.Unmarshal(bytes, &report)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (c *libpodClient) ping(ctx context.Context) error {
	resp, err := c.request(ctx, "/_ping", nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping response was %d", resp.StatusCode)
	}
	return nil
}

func (c *libpodClient) events(ctx context.Context, options url.Values) (<-chan Event, <-chan error) {
	events := make(chan Event)
	errs := make(chan error)
	go func() {
		resp, err := c.request(ctx, "/events", options)
		if err != nil {
			errs <- err
		}
		dec := json.NewDecoder(resp.Body)
		for {
			var e Event
			select {
			case <-ctx.Done():
				return
			default:
				err := dec.Decode(&e)
				if err != nil {
					errs <- err
				} else {
					events <- e
				}
			}
		}
	}()
	return events, errs
}
