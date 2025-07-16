// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"
import (
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

type Config struct {
	Auth        Auth          `mapstructure:"auth"`
	DSN         string        `mapstructure:"dsn"`
	Keyspace    string        `mapstructure:"keyspace"`
	TraceTable  string        `mapstructure:"trace_table"`
	LogsTable   string        `mapstructure:"logs_table"`
	Compression Compression   `mapstructure:"compression"`
	Replication Replication   `mapstructure:"replication"`
	Port        int           `mapstructure:"port"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type Replication struct {
	Class             string `mapstructure:"class"`
	ReplicationFactor int    `mapstructure:"replication_factor"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Compression struct {
	Algorithm string `mapstructure:"algorithm"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Auth struct {
	UserName string              `mapstructure:"username"`
	Password configopaque.String `mapstructure:"password"`
	// prevent unkeyed literal initialization
	_ struct{}
}
