// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"
import (
	"errors"
	"fmt"
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

func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must not be empty")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if cfg.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if cfg.Keyspace == "" {
		return errors.New("keyspace must not be empty")
	}
	if cfg.TraceTable == "" {
		return errors.New("trace_table must not be empty")
	}
	if cfg.LogsTable == "" {
		return errors.New("logs_table must not be empty")
	}
	if err := cfg.Replication.Validate(); err != nil {
		return fmt.Errorf("invalid replication: %w", err)
	}
	if err := cfg.Compression.Validate(); err != nil {
		return fmt.Errorf("invalid compression: %w", err)
	}
	if err := cfg.Auth.Validate(); err != nil {
		return err
	}
	return nil
}

func (r Replication) Validate() error {
	if r.Class == "" {
		return errors.New("class must not be empty")
	}
	if r.ReplicationFactor <= 0 {
		return errors.New("replication_factor must be positive")
	}
	return nil
}

func (c Compression) Validate() error {
	if c.Algorithm == "" {
		return errors.New("algorithm must not be empty")
	}
	return nil
}

func (a Auth) Validate() error {
	if a.UserName != "" && a.Password == "" {
		return errors.New("empty auth.password")
	}
	if a.Password != "" && a.UserName == "" {
		return errors.New("empty auth.username")
	}
	return nil
}
