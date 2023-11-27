// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

type OpenMetadataHostDetails struct {
	Name        string `json:"name"`
	OsName      string `json:"osName"`
	OsVersion   string `json:"osVersion"`
	Environment string `json:"environment"`
}

type OpenMetadataCollectorDetails struct {
	RunningVersion string `json:"runningVersion"`
}

type OpenMetadataNetworkDetails struct {
	HostIpAddress string `json:"hostIpAddress"`
}

type OpenMetadataRequestPayload struct {
	HostDetails      OpenMetadataHostDetails      `json:"hostDetails"`
	CollectorDetails OpenMetadataCollectorDetails `json:"collectorDetails"`
	NetworkDetails   OpenMetadataNetworkDetails   `json:"networkDetails"`
	TagDetails       map[string]interface{}       `json:"tagDetails"`
}
