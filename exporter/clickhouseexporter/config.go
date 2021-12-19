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

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	// Address is the ClickHouse server address.
	Address string `mapstructure:"address"`
	// Database is the database name write data.
	Database string `mapstructure:"database"`
	// Username is the username connect to ClickHouse.
	Username string `mapstructure:"username"`
	// Password is the password connect to ClickHouse.
	Password string `mapstructure:"password"`
	// CaFile is the ca_file connect to ClickHouse.
	CaFile string `mapstructure:"ca_file"`
	// TTLDays is the data ttl days.
	TTLDays uint `mapstructure:"ttl"`
}

var (
	errConfigNoAddress  = errors.New("address must be specified")
	errConfigNoDatabase = errors.New("database must be specified")
)

// Validate validates the clickhouse server configuration.
func (cfg *Config) Validate() error {
	if cfg.Address == "" {
		return errConfigNoAddress
	}
	if cfg.Database == "" {
		return errConfigNoDatabase
	}
	return nil
}
