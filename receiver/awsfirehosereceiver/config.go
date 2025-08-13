// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
)

var errRecordTypeEncodingSet = errors.New("record_type must not be set when encoding is set")

type Config struct {
	// ServerConfig is used to set up the Firehose delivery
	// endpoint. The Firehose delivery stream expects an HTTPS
	// endpoint, so TLSs must be used to enable that.
	confighttp.ServerConfig `mapstructure:",squash"`
	// Encoding identifies the encoding of records received from
	// Firehose. Defaults to telemetry-specific encodings: "cwlog"
	// for logs, and "cwmetrics" for metrics.
	Encoding string `mapstructure:"encoding"`
	// RecordType is an alias for Encoding for backwards compatibility.
	// It is an error to specify both encoding and record_type.
	//
	// Deprecated: [v0.121.0] use Encoding instead.
	RecordType string `mapstructure:"record_type"`
	// AccessKey is checked against the one received with each request.
	// This can be set when creating or updating the Firehose delivery
	// stream.
	AccessKey configopaque.String `mapstructure:"access_key"`
}

// Validate checks that the endpoint and record type exist and
// are valid.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("must specify endpoint")
	}
	if c.RecordType != "" && c.Encoding != "" {
		return errRecordTypeEncodingSet
	}
	return nil
}

func handleDeprecatedConfig(cfg *Config, logger *zap.Logger) {
	if cfg.RecordType != "" {
		logger.Warn("record_type is deprecated, and will be removed in a future version. Use encoding instead.")
	}
}
