// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package test04

type NetworkConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type connectionOptions struct {
	UseTLS       bool `mapstructure:"use_tls"`
	TimeoutInSec int  `mapstructure:"timeout_in_sec"`
}
