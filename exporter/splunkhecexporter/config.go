// Copyright 2019, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"errors"
	"fmt"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	conventions "go.opentelemetry.io/collector/translator/conventions/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// hecPath is the default HEC path on the Splunk instance.
	hecPath                   = "services/collector"
	maxContentLengthLogsLimit = 2 * 1024 * 1024
)

// NewConfig allows to create a config struct and initialize it.
func NewConfig(config *Config) *Config {
	config.initialize()
	return config
}

// Config defines configuration for Splunk exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// HEC Token is the authentication token provided by Splunk: https://docs.splunk.com/Documentation/Splunk/latest/Data/UsetheHTTPEventCollector.
	Token string `mapstructure:"token"`

	// URL is the Splunk HEC endpoint where data is going to be sent to.
	Endpoint string `mapstructure:"endpoint"`

	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourcetype"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open. Defaults to 100.
	MaxConnections uint `mapstructure:"max_connections"`

	// Disable GZip compression. Defaults to false.
	DisableCompression bool `mapstructure:"disable_compression"`

	// Maximum log data size in bytes per HTTP post. Defaults to the backend limit of 2097152 bytes (2MiB).
	MaxContentLengthLogs uint `mapstructure:"max_content_length_logs"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.TLSClientSetting `mapstructure:",squash"`

	// App name is used to track telemetry information for Splunk App's using HEC by App name. Defaults to "OpenTelemetry Collector Contrib".
	SplunkAppName string `mapstructure:"splunk_app_name"`

	// App version is used to track telemetry information for Splunk App's using HEC by App version. Defaults to the current OpenTelemetry Collector Contrib build version.
	SplunkAppVersion string `mapstructure:"splunk_app_version"`

	// SourceKey informs the exporter to map a specific unified model attribute value to the standard source field of a HEC event.
	SourceKey string `mapstructure:"source_key"`
	// SourceTypeKey informs the exporter to map a specific unified model attribute value to the standard sourcetype field of a HEC event.
	SourceTypeKey string `mapstructure:"sourcetype_key"`
	// IndexKey informs the exporter to map the index field to a specific unified model attribute.
	IndexKey string `mapstructure:"index_key"`
	// HostKey informs the exporter to map a specific unified model attribute value to the standard Host field and the host.name field of a HEC event.
	HostKey string `mapstructure:"host_key"`
	// SeverityTextKey informs the exporter to map the severity text field to a specific HEC field.
	SeverityTextKey string `mapstructure:"severity_text_key"`
	// SeverityNumberKey informs the exporter to map the severity number field to a specific HEC field.
	SeverityNumberKey string `mapstructure:"severity_number_key"`
	// NameKey informs the exporter to map the name field to a specific HEC field.
	NameKey string `mapstructure:"name_key"`
}

func (cfg *Config) getOptionsFromConfig() (*exporterOptions, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	url, err := cfg.getURL()
	if err != nil {
		return nil, fmt.Errorf(`invalid "endpoint": %v`, err)
	}

	return &exporterOptions{
		url:   url,
		token: cfg.Token,
	}, nil
}

func (cfg *Config) validateConfig() error {
	if cfg.Endpoint == "" {
		return errors.New(`requires a non-empty "endpoint"`)
	}

	if cfg.Token == "" {
		return errors.New(`requires a non-empty "token"`)
	}

	if cfg.MaxContentLengthLogs > maxContentLengthLogsLimit {
		return fmt.Errorf(`requires "max_content_length_logs" <= %d`, maxContentLengthLogsLimit)
	}

	return nil
}

func (cfg *Config) getURL() (out *url.URL, err error) {

	out, err = url.Parse(cfg.Endpoint)
	if err != nil {
		return out, err
	}
	if out.Path == "" || out.Path == "/" {
		out.Path = path.Join(out.Path, hecPath)
	}

	return
}

func (cfg *Config) GetSourceKey() string {
	return cfg.SourceKey
}

func (cfg *Config) GetSourceTypeKey() string {
	return cfg.SourceTypeKey
}

func (cfg *Config) GetIndexKey() string {
	return cfg.IndexKey
}

func (cfg *Config) GetHostKey() string {
	return cfg.HostKey
}

func (cfg *Config) GetNameKey() string {
	return cfg.NameKey
}

func (cfg *Config) GetSeverityTextKey() string {
	return cfg.SeverityTextKey
}

func (cfg *Config) GetSeverityNumberKey() string {
	return cfg.SeverityNumberKey
}

// initialize the configuration
func (cfg *Config) initialize() {
	if cfg.SourceKey == "" {
		cfg.SourceKey = splunk.DefaultSourceLabel
	}
	if cfg.SourceTypeKey == "" {
		cfg.SourceTypeKey = splunk.DefaultSourceTypeLabel
	}
	if cfg.IndexKey == "" {
		cfg.IndexKey = splunk.DefaultIndexLabel
	}
	if cfg.HostKey == "" {
		cfg.HostKey = conventions.AttributeHostName
	}
	if cfg.SeverityTextKey == "" {
		cfg.SeverityTextKey = splunk.DefaultSeverityTextLabel
	}
	if cfg.SeverityNumberKey == "" {
		cfg.SeverityNumberKey = splunk.DefaultSeverityNumberLabel
	}
	if cfg.NameKey == "" {
		cfg.NameKey = splunk.DefaultNameLabel
	}
}
