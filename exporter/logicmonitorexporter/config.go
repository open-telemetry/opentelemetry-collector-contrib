// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for LogicMonitor exporter.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`

	QueueSettings               exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig   `mapstructure:"retry_on_failure"`
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ApiToken of Logicmonitor Platform
	APIToken APIToken `mapstructure:"api_token"`
	// Logs defines the Logs exporter specific configuration
	Logs LogsConfig `mapstructure:"logs"`
}

type APIToken struct {
	AccessID  string              `mapstructure:"access_id"`
	AccessKey configopaque.String `mapstructure:"access_key"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type MappingOperation string

const (
	And MappingOperation = "and"
	Or  MappingOperation = "or"
)

func (mop *MappingOperation) UnmarshalText(in []byte) error {
	switch op := MappingOperation(strings.ToLower(string(in))); op {
	case And, Or:
		*mop = op
		return nil

	default:
		return fmt.Errorf("unsupported mapping operation %q", op)
	}
}

// LogsConfig defines the logs exporter specific configuration options
type LogsConfig struct {
	// Operation to be performed for resource mapping. Valid values are `and`, `or`.
	ResourceMappingOperation MappingOperation `mapstructure:"resource_mapping_op"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint should not be empty")
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("endpoint must be valid")
	}
	return nil
}
