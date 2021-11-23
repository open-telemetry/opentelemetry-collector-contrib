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

package podmanreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

type containerStats struct {
	AvgCPU        float64
	ContainerID   string
	Name          string
	PerCPU        []uint64
	CPU           float64
	CPUNano       uint64
	CPUSystemNano uint64
	DataPoints    int64
	SystemNano    uint64
	MemUsage      uint64
	MemLimit      uint64
	MemPerc       float64
	NetInput      uint64
	NetOutput     uint64
	BlockInput    uint64
	BlockOutput   uint64
	PIDs          uint64
	UpTime        time.Duration
	Duration      uint64
}

type containerStatsReport struct {
	Error string
	Stats []containerStats
}

type clientFactory func(logger *zap.Logger, cfg *Config) (client, error)

type client interface {
	stats() ([]containerStats, error)
}

type podmanClient struct {
	conn     *http.Client
	endpoint string
}

func newPodmanClient(logger *zap.Logger, cfg *Config) (client, error) {
	connection, err := newPodmanConnection(logger, cfg.Endpoint, cfg.SSHKey, cfg.SSHPassphrase)
	if err != nil {
		return nil, err
	}
	c := &podmanClient{
		conn:     connection,
		endpoint: fmt.Sprintf("http://d/v%s/libpod", cfg.APIVersion),
	}
	err = c.ping()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *podmanClient) request(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}

	return c.conn.Do(req)
}

func (c *podmanClient) stats() ([]containerStats, error) {
	params := url.Values{}
	params.Add("stream", "false")

	resp, err := c.request(context.Background(), "/containers/stats", params)
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

func (c *podmanClient) ping() error {
	resp, err := c.request(context.Background(), "/_ping", nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping response was %d", resp.StatusCode)
	}
	return nil
}
