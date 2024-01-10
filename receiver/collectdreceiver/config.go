// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for Collectd receiver.
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	Timeout                       time.Duration            `mapstructure:"timeout"`
	Encoding                      string                   `mapstructure:"encoding"`
	AttributesPrefix              string                   `mapstructure:"attributes_prefix"`
}

func (c *Config) Validate() error {
	// CollectD receiver only supports JSON encoding. We expose a config option
	// to make it explicit and obvious to the users.
	if strings.ToLower(c.Encoding) != defaultEncodingFormat {
		return fmt.Errorf(
			"CollectD only support JSON encoding format. %s is not supported",
			c.Encoding,
		)
	}
	return nil
}
