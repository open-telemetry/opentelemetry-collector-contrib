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
	"io"
	"io/ioutil"
	"log"
	"math"
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

type Type = string

type Actor struct {
	ID         string
	Attributes map[string]string
}

type Event struct {
	Status string `json:"status,omitempty"`
	ID     string `json:"id,omitempty"`
	From   string `json:"from,omitempty"`

	Type   string
	Action string
	Actor  Actor

	Scope string `json:"scope,omitempty"`

	Time     int64 `json:"time,omitempty"`
	TimeNano int64 `json:"timeNano,omitempty"`

	Error string
}

type clientFactory func(logger *zap.Logger, cfg *Config) (client, error)

type client interface {
	stats() ([]containerStats, error)
	events() (chan Event, error)
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
func (c *podmanClient) events() (chan Event, error) {
	ch := make(chan Event)
	params := url.Values{}
	params.Add("stream", "true")
	params.Add("since", "0m")

	response, err := c.request(context.Background(), "/events", params)
	if err != nil {
		return nil, err
	}

	go func() {
		dec := json.NewDecoder(response.Body)

		shouldRetry := true
		for retries := 1; retries <= 3 && shouldRetry; retries++ {
			if retries != 1 {
				fmt.Println("Retrying")
			}
			for {
				var event Event
				if err := dec.Decode(&event); err != nil {
					if err == io.EOF {
						break
					}
					shouldRetry = true
					break
				}
				shouldRetry = false
				ch <- event
			}
			response.Body.Close()
			delay := math.Pow(2, float64(retries))
			time.Sleep(time.Duration(delay) * time.Second)
		}
		if shouldRetry {
			log.Fatal("Error while decoding events")
		}
	}()
	return ch, nil
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
