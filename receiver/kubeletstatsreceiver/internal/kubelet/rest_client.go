// Copyright The OpenTelemetry Authors
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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
)

// RestClient is swappable for testing.
type RestClient interface {
	StatsSummary() ([]byte, error)
	Pods() ([]byte, error)
}

// HTTPRestClient is a thin wrapper around a kubelet client, encapsulating endpoints
// and their corresponding http methods. The endpoints /stats/container /spec/
// are excluded because they require cadvisor. The /metrics endpoint is excluded
// because it returns Prometheus data.
type HTTPRestClient struct {
	client kube.Client
}

func NewRestClient(client kube.Client) *HTTPRestClient {
	return &HTTPRestClient{client: client}
}

func (c *HTTPRestClient) StatsSummary() ([]byte, error) {
	return c.client.Get("/stats/summary")
}

func (c *HTTPRestClient) Pods() ([]byte, error) {
	return c.client.Get("/pods")
}
