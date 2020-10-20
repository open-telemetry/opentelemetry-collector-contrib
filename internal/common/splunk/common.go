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
	HECTokenHeader        = "Splunk"
	HecTokenLabel         = "com.splunk.hec.access_token"
	// HecEventMetricType is the type of HEC event. Set to metric, as per https://docs.splunk.com/Documentation/Splunk/8.0.3/Metrics/GetMetricsInOther.
	HecEventMetricType = "metric"
)

// AccessTokenPassthroughConfig configures passing through access tokens.
type AccessTokenPassthroughConfig struct {
	// AccessTokenPassthrough indicates whether to associate datapoints with an organization access token received in request.
	AccessTokenPassthrough bool `mapstructure:"access_token_passthrough"`
}

// Event represents a metric in Splunk HEC format
type Event struct {
	Time       float64                `json:"time"`                 // epoch time
	Host       string                 `json:"host"`                 // hostname
	Source     string                 `json:"source,omitempty"`     // optional description of the source of the event; typically the app's name
	SourceType string                 `json:"sourcetype,omitempty"` // optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	Index      string                 `json:"index,omitempty"`      // optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Event      interface{}            `json:"event"`                // type of event: set to "metric" or nil if the event represents a metric, or is the payload of the event.
	Fields     map[string]interface{} `json:"fields,omitempty"`     // dimensions and metric data
}

// IsMetric returns true if the Splunk event is a metric.
func (m Event) IsMetric() bool {
	return m.Event == HecEventMetricType || (m.Event == nil && len(m.GetMetricValues()) > 0)
}

// GetMetricValues extracts metric key value pairs from a Splunk HEC metric.
func (m Event) GetMetricValues() map[string]interface{} {
	values := map[string]interface{}{}
	for k, v := range m.Fields {
		if strings.HasPrefix(k, "metric_name:") {
			values[k[12:]] = v
		}
	}
	return values
}
