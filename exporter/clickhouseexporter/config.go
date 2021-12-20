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
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// Address is the ClickHouse server address.
	Address string `mapstructure:"address"`
	// Database is the database name write data.
	Database string `mapstructure:"database"`
	// Username is the username to connect ClickHouse.
	Username string `mapstructure:"username"`
	// Password is the password to connect ClickHouse.
	Password string `mapstructure:"password"`
	// CaFile is the cert file path to connect ClickHouse.
	CaFile string `mapstructure:"ca_file"`
	// TTLDays is The data time-to-live in days, 0 means no ttl.
	TTLDays uint `mapstructure:"ttl"`
}

var (
	errConfigNoAddress  = errors.New("address must be specified")
	errConfigNoDatabase = errors.New("database must be specified")
)

// Validate validates the clickhouse server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.Address == "" {
		err = multierr.Append(err, errConfigNoAddress)
	}
	if cfg.Database == "" {
		err = multierr.Append(err, errConfigNoDatabase)
	}
	return err
}
