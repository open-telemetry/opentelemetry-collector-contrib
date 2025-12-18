// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

type RefFieldConfig struct {
	Database DatabaseConfig `mapstructure:"database" description:"Database configuration"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host" description:"Database host"`
	Port     int    `mapstructure:"port" description:"Database port"`
	Username string `mapstructure:"username" description:"Database username"`
	Password string `mapstructure:"password" description:"Database password"`
}
