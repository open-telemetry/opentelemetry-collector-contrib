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

package splunk

import "strings"

const (
	SFxAccessTokenHeader  = "X-Sf-Token"
	SFxAccessTokenLabel   = "com.splunk.signalfx.access_token"
	SFxEventCategoryKey   = "com.splunk.signalfx.event_category"
	SFxEventPropertiesKey = "com.splunk.signalfx.event_properties"
	SourcetypeLabel       = "com.splunk.sourcetype"
)

type AccessTokenPassthroughConfig struct {
	// Whether to associate datapoints with an organization access token received in request.
	AccessTokenPassthrough bool `mapstructure:"access_token_passthrough"`
}

// Metric represents a metric in Splunk HEC format
type Metric struct {
	Time       float64                `json:"time"`                 // epoch time
	Host       string                 `json:"host"`                 // hostname
	Source     string                 `json:"source,omitempty"`     // optional description of the source of the event; typically the app's name
	SourceType string                 `json:"sourcetype,omitempty"` // optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	Index      string                 `json:"index,omitempty"`      // optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Event      string                 `json:"event"`                // type of event: this is a metric.
	Fields     map[string]interface{} `json:"fields"`               // metric data
}

// GetValues extracts metric key value pairs from a Splunk HEC metric.
func (m Metric) GetValues() map[string]interface{} {
	values := map[string]interface{}{}
	for k, v := range m.Fields {
		if strings.HasPrefix(k, "metric_name:") {
			values[k[12:]] = v
		}
	}
	return values
}
