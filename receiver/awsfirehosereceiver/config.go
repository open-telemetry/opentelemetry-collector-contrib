// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	// HTTPServerSettings is used to set up the Firehose delivery
	// endpoint. The Firehose delivery stream expects an HTTPS
	// endpoint, so TLSSettings must be used to enable that.
	confighttp.HTTPServerSettings `mapstructure:",squash"`
	// RecordType is the key used to determine which unmarshaler to use
	// when receiving the requests.
	RecordType string `mapstructure:"record_type"`
	// AccessKey is checked against the one received with each request.
	// This can be set when creating or updating the Firehose delivery
	// stream.
	AccessKey string `mapstructure:"access_key"`
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
	return nil
}
