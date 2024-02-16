// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Constants for Splunk components.
const (
	SFxAccessTokenHeader       = "X-Sf-Token"                       // #nosec
	SFxAccessTokenLabel        = "com.splunk.signalfx.access_token" // #nosec
	SFxEventCategoryKey        = "com.splunk.signalfx.event_category"
	SFxEventPropertiesKey      = "com.splunk.signalfx.event_properties"
	SFxEventType               = "com.splunk.signalfx.event_type"
	DefaultSourceTypeLabel     = "com.splunk.sourcetype"
	DefaultSourceLabel         = "com.splunk.source"
	DefaultIndexLabel          = "com.splunk.index"
	DefaultNameLabel           = "otel.log.name"
	DefaultSeverityTextLabel   = "otel.log.severity.text"
	DefaultSeverityNumberLabel = "otel.log.severity.number"
	HECTokenHeader             = "Splunk"
	HecTokenLabel              = "com.splunk.hec.access_token" // #nosec
	// HecEventMetricType is the type of HEC event. Set to metric, as per https://docs.splunk.com/Documentation/Splunk/8.0.3/Metrics/GetMetricsInOther.
	HecEventMetricType = "metric"
	DefaultRawPath     = "/services/collector/raw"
	DefaultHealthPath  = "/services/collector/health"
)

// AccessTokenPassthroughConfig configures passing through access tokens.
type AccessTokenPassthroughConfig struct {
	// AccessTokenPassthrough indicates whether to associate datapoints with an organization access token received in request.
	AccessTokenPassthrough bool `mapstructure:"access_token_passthrough"`
}

// Event represents a metric in Splunk HEC format
type Event struct {
	Time       float64        `json:"time,omitempty"`       // optional epoch time - set to zero if the event timestamp is missing or unknown (will be added at indexing time)
	Host       string         `json:"host"`                 // hostname
	Source     string         `json:"source,omitempty"`     // optional description of the source of the event; typically the app's name
	SourceType string         `json:"sourcetype,omitempty"` // optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	Index      string         `json:"index,omitempty"`      // optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Event      any            `json:"event"`                // type of event: set to "metric" or nil if the event represents a metric, or is the payload of the event.
	Fields     map[string]any `json:"fields,omitempty"`     // dimensions and metric data
}

// IsMetric returns true if the Splunk event is a metric.
func (e Event) IsMetric() bool {
	return e.Event == HecEventMetricType || (e.Event == nil && len(e.GetMetricValues()) > 0)
}

// GetMetricValues extracts metric key value pairs from a Splunk HEC metric.
func (e Event) GetMetricValues() map[string]any {
	values := map[string]any{}
	for k, v := range e.Fields {
		if strings.HasPrefix(k, "metric_name:") {
			values[k[12:]] = v
		}
	}
	return values
}

// UnmarshalJSON unmarshals the JSON representation of an event
func (e *Event) UnmarshalJSON(b []byte) error {
	rawEvent := struct {
		Time       any            `json:"time,omitempty"`
		Host       string         `json:"host"`
		Source     string         `json:"source,omitempty"`
		SourceType string         `json:"sourcetype,omitempty"`
		Index      string         `json:"index,omitempty"`
		Event      any            `json:"event"`
		Fields     map[string]any `json:"fields,omitempty"`
	}{}
	err := json.Unmarshal(b, &rawEvent)
	if err != nil {
		return err
	}
	*e = Event{
		Host:       rawEvent.Host,
		Source:     rawEvent.Source,
		SourceType: rawEvent.SourceType,
		Index:      rawEvent.Index,
		Event:      rawEvent.Event,
		Fields:     rawEvent.Fields,
	}
	switch t := rawEvent.Time.(type) {
	case float64:
		e.Time = t
	case string:
		{
			time, err := strconv.ParseFloat(t, 64)
			if err != nil {
				return err
			}
			e.Time = time
		}
	}
	return nil
}

// HecToOtelAttrs defines the mapping of Splunk HEC metadata to attributes
type HecToOtelAttrs struct {
	// Source indicates the mapping of the source field to a specific unified model attribute.
	Source string `mapstructure:"source"`
	// SourceType indicates the mapping of the sourcetype field to a specific unified model attribute.
	SourceType string `mapstructure:"sourcetype"`
	// Index indicates the mapping of the index field to a specific unified model attribute.
	Index string `mapstructure:"index"`
	// Host indicates the mapping of the host field to a specific unified model attribute.
	Host string `mapstructure:"host"`
}
