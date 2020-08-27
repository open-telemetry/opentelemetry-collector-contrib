// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"io/ioutil"
)

// RestClient is swappable for testing.
type RestClient interface {
	EndpointResponse() ([]byte, []byte, error)
}

// RestClient is a thin wrapper around an ecs task metadata client, encapsulating endpoints
// and their corresponding http methods.
type HTTPRestClient struct {
	client Client
}

// Creates a new copy of the Rest Client
func NewRestClient(client Client) *HTTPRestClient {
	return &HTTPRestClient{client: client}
}

// Gets the task metadata and docker stats from ECS Task Metadata
func (c *HTTPRestClient) EndpointResponse() ([]byte, []byte, error) {
	taskStats, err := ioutil.ReadFile("/Users/hrayhan/Work/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/testdata/task_stats.json")
	if err != nil {
		return nil, nil, err
	}
	taskMetadata, err := ioutil.ReadFile("/Users/hrayhan/Work/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/testdata/task_metadata.json")
	if err != nil {
		return nil, nil, err
	}
	return taskStats, taskMetadata, nil
	//return ioutil.ReadFile("/Users/hrayhan/Work/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/testdata/task_stats.json")
	// return c.client.Get("/stats/summary")
}
