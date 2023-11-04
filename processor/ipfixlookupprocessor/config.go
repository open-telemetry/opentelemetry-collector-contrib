// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import "fmt"

type ElasticsearchConnection struct {
	Addresses              []string `mapstructure:"addresses"`
	Username               string   `mapstructure:"username"`
	Password               string   `mapstructure:"password"`
	CertificateFingerprint string   `mapstructure:"certificate_fingerprint"`
}

type ElasticsearchConfig struct {
	Connection ElasticsearchConnection `mapstructure:"connection"`
}

type LookupFields struct {
	SourceIP        string `mapstructure:"source_ip"`
	SourcePort      string `mapstructure:"source_port"`
	DestinationIP   string `mapstructure:"destination_ip"`
	DestinationPort string `mapstructure:"destination_port"`
}

type BaseQuery struct {
	FieldName    string       `mapstructure:"field_name"`
	FieldValue   string       `mapstructure:"field_value"`
	LookupFields LookupFields `mapstructure:"lookup_fields"`
}

type QueryParameters struct {
	BaseQuery        BaseQuery    `mapstructure:"base_query"`
	DeviceIdentifier string       `mapstructure:"device_identifier"`
	LookupFields     LookupFields `mapstructure:"lookup_fields"`
}

type TimingConfig struct {
	LookupWindow int `mapstructure:"lookup_window"`
}

type SpanFields struct {
	SourceIPs            []string `mapstructure:"source_ips"`
	SourcePorts          []string `mapstructure:"source_ports"`
	DestinationIPandPort []string `mapstructure:"destination_ip_and_port"`
	DestinationIPs       []string `mapstructure:"destination_ips"`
	DestinationPorts     []string `mapstructure:"destination_ports"`
}

type Spans struct {
	SpanFields SpanFields `mapstructure:"span_fields"`
}

type Config struct {
	Elasticsearch       ElasticsearchConfig `mapstructure:"elastic_search"`
	QueryParameters     QueryParameters     `mapstructure:"query_parameters"`
	SpanAttributeFields []string            `mapstructure:"span_attribute_fields"`
	Timing              TimingConfig        `mapstructure:"timing"`
	Spans               Spans               `mapstructure:"spans"`
}

func (c *Config) Validate() error {
	// Validate Elasticsearch fields
	if len(c.Elasticsearch.Connection.Addresses) == 0 {
		return fmt.Errorf("elasticsearch addresses must not be empty")
	}
	if c.Elasticsearch.Connection.Username == "" {
		return fmt.Errorf("elasticsearch username must not be empty")
	}
	if c.Elasticsearch.Connection.Password == "" {
		return fmt.Errorf("elasticsearch password must not be empty")
	}
	if c.Elasticsearch.Connection.CertificateFingerprint == "" {
		return fmt.Errorf("elasticsearch certificateFingerprint must not be empty")
	}

	// Validate QueryParameters fields
	if c.QueryParameters.DeviceIdentifier == "" {
		return fmt.Errorf("queryParameters deviceIdentifier must not be empty")
	}
	if c.QueryParameters.BaseQuery.FieldName == "" || c.QueryParameters.BaseQuery.FieldValue == "" {
		return fmt.Errorf("queryParameters baseQuery fieldName and fieldValue must not be empty")
	}

	// Validate SpanAttributeFields
	if len(c.SpanAttributeFields) == 0 {
		return fmt.Errorf("spanAttributeFields must not be empty")
	}

	// Validate Spans fields
	if len(c.Spans.SpanFields.SourceIPs) == 0 || len(c.Spans.SpanFields.DestinationIPs) == 0 {
		return fmt.Errorf("spans sourceIPs and destinationIPs must not be empty")
	}

	// Validate timing fields
	if c.Timing.LookupWindow < 0 {
		return fmt.Errorf("lookupWindow must be greater than 0")
	}
	return nil
}
