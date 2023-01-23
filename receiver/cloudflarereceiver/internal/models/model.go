// Copyright The OpenTelemetry Authors
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

// Package models contains the Log struct returned from Cloudflare api.
package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"

// Log holds the supported log fields.
// A full list of all logs fields can be found at https://developers.cloudflare.com/logs/reference/log-fields/zone/http_requests
type Log struct {
	ClientIP            string `json:"ClientIP"`
	ClientRequestHost   string `json:"ClientRequestHost"`
	ClientRequestMethod string `json:"ClientRequestMethod"`
	ClientRequestURI    string `json:"ClientRequestURI"`
	EdgeEndTimestamp    int64  `json:"EdgeEndTimestamp"`
	EdgeResponseBytes   int64  `json:"EdgeResponseBytes"`
	EdgeResponseStatus  int64  `json:"EdgeResponseStatus"`
	EdgeStartTimestamp  int64  `json:"EdgeStartTimestamp"`
	RayID               string `json:"RayID"`
}

// Response holds the raw return response from using the cloudflare API.
type Response struct {
	Errors   []interface{} `json:"errors"`
	Messages []interface{} `json:"messages"`
	Result   []Log         `json:"result"`
	Success  bool          `json:"success"`
}
