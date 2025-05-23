// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Copyright © 2025, Oracle and/or its affiliates.

package oracleobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type AuthenticationType string

const (
	ConfigFile        AuthenticationType = "config_file"
	InstancePrincipal AuthenticationType = "instance_principal"
)

type OciConfig struct {
	// The fingerprint of the OCI user.
	FingerPrint configopaque.String `mapstructure:"fingerprint"`

	// The private key of the OCI user.
	PrivateKey configopaque.String `mapstructure:"private_key"`

	// The OCI tenancy OCID.
	Tenancy configopaque.String `mapstructure:"tenancy"`

	// The OCI region.
	Region configopaque.String `mapstructure:"region"`

	// The OCI user OCID.
	User configopaque.String `mapstructure:"user"`
}

// Config defines configuration settings for the Oracle Observability exporter.
type Config struct {
	TimeoutConfig exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	QueueConfig   exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	BackOffConfig configretry.BackOffConfig       `mapstructure:"retry_on_failure"`

	// The authentication type to use. Supported values are: config_file or instance_principal, default is config_file.
	AuthType AuthenticationType `mapstructure:"auth_type"`

	// The OCI tenancy namespace to which the collected log data will be uploaded.
	NamespaceName string `mapstructure:"namespace"`

	// The Logging Analytics log group OCID to which the log data will be mapped. This is mainly used for AuthZ purpose.
	LogGroupID string `mapstructure:"log_group_id"`

	// Path to the OCI configuration file, if auth_type is config_file.
	// If auth_type is config_file and this is not provided, the exporter will search for the configuration in the default location
	// ($HOME/.oci/config or the location specified by the OCI_CONFIG_FILE environment variable).
	ConfigFilePath configopaque.String `mapstructure:"oci_config_file_path"`

	// The profile to be used in the OCI configuration file, if auth_type is config_file
	ConfigProfile configopaque.String `mapstructure:"config_profile"`

	// Passphrase for the private key used in the config file, if the key is password-protected.
	// Only applicable when `auth_type` is `config_file`.
	PrivateKeyPassphrase configopaque.String `mapstructure:"private_key_passphrase"`

	// An alternative way to provide the content of the OCI Config file, if auth_type is config_file.
	OciConfiguration OciConfig `mapstructure:"oci_config"`
}

// Validate checks if the exporter configuration is valid
var _ component.Config = (*Config)(nil)

func isOciConfigUsed(ociConfiguration OciConfig) bool {
	if ociConfiguration.FingerPrint != "" || ociConfiguration.PrivateKey != "" || ociConfiguration.Region != "" ||
		ociConfiguration.Tenancy != "" || ociConfiguration.User != "" {
		return true
	}
	return false
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return errors.New("missing configuration, you must provide a valid configuration for the Oracle Observability exporter")
	}
	if cfg.NamespaceName == "" {
		return errors.New("'namespace' is a required field. You may find using OCI Console under Logging Analytics → Administration → Service")
	}
	if cfg.LogGroupID == "" {
		return errors.New("'log_group_id' is a required field")
	}
	if cfg.AuthType != ConfigFile && cfg.AuthType != InstancePrincipal {
		return errors.New("invalid 'auth_type', supported values are 'config_file' and 'instance_principal'")
	}

	isOciConfigUsed := isOciConfigUsed(cfg.OciConfiguration)

	if cfg.AuthType != ConfigFile && isOciConfigUsed {
		return errors.New("'oci_config' field is not applicable when 'auth_type' is set to 'instance_principal'")
	}

	if isOciConfigUsed {
		if cfg.OciConfiguration.FingerPrint == "" {
			return errors.New("'fingerprint' can not be empty")
		}
		if cfg.OciConfiguration.PrivateKey == "" {
			return errors.New("'private_key' can not be empty")
		}
		if cfg.OciConfiguration.Region == "" {
			return errors.New("'region' can not be empty")
		}
		if cfg.OciConfiguration.Tenancy == "" {
			return errors.New("'tenancy' can not be empty")
		}
		if cfg.OciConfiguration.User == "" {
			return errors.New("'user' can not be empty")
		}
	}

	if cfg.QueueConfig.BlockOnOverflow {
		fmt.Println("BlockOnOverflow in QueueConfig is set to true; whenever the queue is full, it will wait until enough space is available to add a new request, ensuring no data is dropped but potentially stalling upstream components")
	} else {
		fmt.Println("BlockOnOverflow in QueueConfig is set to false; the queue will discard new requests when it reaches its maximum capacity instead of waiting for space to become available, prioritizing throughput over data reliability")
	}

	return nil
}
