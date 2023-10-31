// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for LogicMonitor exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
	ResourceToTelemetrySettings  resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ApiToken of Logicmonitor Platform
	APIToken APIToken `mapstructure:"api_token"`
}

type APIToken struct {
	AccessID  string              `mapstructure:"access_id"`
	AccessKey configopaque.String `mapstructure:"access_key"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("Endpoint should not be empty")
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("Endpoint must be valid")
	}
	return nil
}
