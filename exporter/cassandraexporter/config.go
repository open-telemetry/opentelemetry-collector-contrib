// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"
import "time"

type Config struct {
	DSN         string        `mapstructure:"dsn"`
	Port        int           `mapstructure:"port"`
	Timeout     time.Duration `mapstructure:"timeout"`
	Keyspace    string        `mapstructure:"keyspace"`
	TraceTable  string        `mapstructure:"trace_table"`
	LogsTable   string        `mapstructure:"logs_table"`
	Replication Replication   `mapstructure:"replication"`
	Compression Compression   `mapstructure:"compression"`
}

type Replication struct {
	Class             string `mapstructure:"class"`
	ReplicationFactor int    `mapstructure:"replication_factor"`
}

type Compression struct {
	Algorithm string `mapstructure:"algorithm"`
}
