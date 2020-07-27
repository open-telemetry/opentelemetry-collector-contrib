// // Copyright 2020, OpenTelemetry Authors
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

package awsecscontainermetrics

// // RestClient is swappable for testing.
// type RestClient interface {
// 	StatsSummary() ([]byte, error)
// }

// // RestClient is a thin wrapper around an ecs task metadata client, encapsulating endpoints
// // and their corresponding http methods. The endpoints /stats/container /spec/
// // are excluded because they require cadvisor. The /metrics endpoint is excluded
// // because it returns Prometheus data.
// type HTTPRestClient struct {
// 	client Client
// }

// // Creates a new copy of the Rest Client
// func NewRestClient(client Client) *HTTPRestClient {
// 	return &HTTPRestClient{client: client}
// }

// // Gets the task metadata and docker stats from ECS Task Metadata
// func (c *HTTPRestClient) StatsSummary() ([]byte, error) {
// 	return c.client.Get("/stats/summary")
// }
