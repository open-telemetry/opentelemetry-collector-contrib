// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

type Config struct {
	// ServerConfig is used to set up the Firehose delivery
	// endpoint. The Firehose delivery stream expects an HTTPS
	// endpoint, so TLSSettings must be used to enable that.
	confighttp.ServerConfig `mapstructure:",squash"`
	// RecordType is the key used to determine which unmarshaler to use
	// when receiving the requests.
	RecordType string `mapstructure:"record_type"`
	// AccessKey is checked against the one received with each request.
	// This can be set when creating or updating the Firehose delivery
	// stream.
	AccessKey configopaque.String `mapstructure:"access_key"`
	// NamePrefixes is a list of attributes that are used to determine
	// the name of the metric. If the attribute is not found in the
	// record, or its string value is empty, the default value is used.
	// Fields are applied in order, and sepatated by a period.
	// Currently, only resource attributes are supported.
	NamePrefixes []NamePrefixConfig `mapstructure:"name_prefixes"`
}

type NamePrefixConfig struct {
	// AttributeName is the name of the attribute in the record.
	AttributeName string `mapstructure:"attribute_name"`
	// Default is the value to use if the attribute is not found or
	// is empty.
	Default string `mapstructure:"default"`
}

// Validate checks that the endpoint and record type exist and
// are valid.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("must specify endpoint")
	}
	if c.RecordType == "" {
		return errors.New("must specify record type")
	}
	if err := validateRecordType(c.RecordType); err != nil {
		return err
	}

	return validateNamePrefixes(c.NamePrefixes)
}

func validateNamePrefixes(namePrefixes []NamePrefixConfig) error {
	for _, namePrefix := range namePrefixes {
		if namePrefix.AttributeName == "" {
			return errors.New("name_prefixes must specify attribute_name")
		}
		if namePrefix.Default == "" {
			return errors.New("name_prefixes must specify default")
		}
	}
	return nil
}
