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

type OpenRegisterRequestPayload struct {
	CollectorName string                 `json:"collectorName"`
	Ephemeral     bool                   `json:"ephemeral,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Hostname      string                 `json:"hostname,omitempty"`
	Category      string                 `json:"category,omitempty"`
	TimeZone      string                 `json:"timeZone,omitempty"`
	Clobber       bool                   `json:"clobber,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

type OpenRegisterResponsePayload struct {
	CollectorCredentialId  string `json:"collectorCredentialId"`
	CollectorCredentialKey string `json:"collectorCredentialKey"`
	CollectorId            string `json:"collectorId"`
	CollectorName          string `json:"collectorName"`
}
