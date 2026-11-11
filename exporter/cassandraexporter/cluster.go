// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"errors"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

func newCluster(cfg *Config) (*gocql.ClusterConfig, error) {
	cluster := gocql.NewCluster(cfg.DSN)
	if cfg.Auth.UserName != "" && cfg.Auth.Password == "" {
		return nil, errors.New("empty auth.password")
	}
	if cfg.Auth.Password != "" && cfg.Auth.UserName == "" {
		return nil, errors.New("empty auth.username")
	}
	if cfg.Auth.UserName != "" && cfg.Auth.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.Auth.UserName,
			Password: string(cfg.Auth.Password),
		}
	}
	cluster.Consistency = gocql.Quorum
	cluster.Port = cfg.Port
	cluster.Timeout = cfg.Timeout
	return cluster, nil
}

func newDataSession(cfg *Config) (*gocql.Session, error) {
	cluster, err := newCluster(cfg)
	if err != nil {
		return nil, err
	}
	cluster.Keyspace = cfg.Keyspace
	return cluster.CreateSession()
}
