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

package syslogexporter

import (
	"errors"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"net/url"
	"os"
)

// Config defines configuration for Syslog exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	//todo doc
	Endpoint string `mapstructure:"endpoint"`

	//todo doc
	NetProtocol string `mapstructure:"net_protocol"`

	Retry exporterhelper.RetrySettings `mapstructure:"retry"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`

	// SDID where all paremeters will be added, by default "meta"
	SdConfig SdConfig `mapstructure:"sd_config"`
}

type SdConfig struct {
	// SDID where all paremeters will be added, by default "meta"
	CommonSdId string `mapstructure:"common_sdid"`

	// SDID where trace and span parameter will go, by default "meta"
	TraceSdId string `mapstructure:"trace_sdid"`

	// todo desc key - propery, value - sd_id
	CustomMapping map[string]string `mapstructure:"mapping_sdid"`

	// todo desc Add property to each message
	// todo add doc example
	StaticSd map[string]map[string]string `mapstructure:"static_sd"`
}

const defaultSyslogPort = "514"
const defaultSyslogEndpointEnvName = "SYSLOG_HOST"

var (
	errConfigNoEndpoint    = errors.New("endpoint must be specified")
	errConfigParseEndpoint = errors.New("endpoint parse failed, expected format host:port")
)

func (cfg *Config) setDefaults() error {
	if cfg.Endpoint == "" {
		endpointFromEnv := os.Getenv(defaultSyslogEndpointEnvName)
		if endpointFromEnv == "" {
			return errConfigNoEndpoint
		} else {
			cfg.Endpoint = endpointFromEnv
		}
	}

	parse, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return errConfigParseEndpoint
	}

	if parse.Port() == "" {
		cfg.Endpoint += ":" + defaultSyslogPort
	}

	if cfg.NetProtocol == "" {
		cfg.NetProtocol = "tcp"
	}

	if cfg.SdConfig.CommonSdId == "" {
		cfg.SdConfig.CommonSdId = "meta"
	}

	if cfg.SdConfig.TraceSdId == "" {
		cfg.SdConfig.TraceSdId = cfg.SdConfig.CommonSdId
	}

	return nil
}

func (cfg *Config) Validate() error {
	err := cfg.setDefaults()
	if err != nil {
		return err
	}

	if cfg.Endpoint == "" {
		return errConfigNoEndpoint
	}

	return nil
}
