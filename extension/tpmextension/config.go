// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpmextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// The path to the TPM device or Unix domain socket.
	// For instance /dev/tpm0 or /dev/tpmrm0.
	Path string `mapstructure:"path"`
	// TSS2 key file
	ClientKeyFile  string `mapstructure:"key_file"`
	ClientCertFile string `mapstructure:"cert_file"`
	CaFile         string `mapstructure:"ca_file"`
	ServerName     string `mapstructure:"server_name_override"`

	OwnerAuth string `mapstructure:"owner_auth"`
	Auth      string `mapstructure:"auth"`
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (cfg *Config) Validate() error {
	if cfg.Path == "" {
		return errors.New("path must be non-empty")
	}
	return nil
}
