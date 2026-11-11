// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"fmt"
	"strings"
)

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func parseCreateDatabaseSQL(cfg *Config) string {
	return fmt.Sprintf(createDatabaseSQL, quoteIdent(cfg.Keyspace), cfg.Replication.Class, cfg.Replication.ReplicationFactor)
}

func parseCreateEventsTypeSQL(cfg *Config) string {
	return fmt.Sprintf(createEventTypeSQL, quoteIdent(cfg.Keyspace))
}

func parseCreateLinksTypeSQL(cfg *Config) string {
	return fmt.Sprintf(createLinksTypeSQL, quoteIdent(cfg.Keyspace))
}

func parseCreateSpanTableSQL(cfg *Config) string {
	return fmt.Sprintf(createSpanTableSQL, quoteIdent(cfg.Keyspace), quoteIdent(cfg.TraceTable), cfg.Compression.Algorithm)
}

func parseCreateLogTableSQL(cfg *Config) string {
	return fmt.Sprintf(createLogTableSQL, quoteIdent(cfg.Keyspace), quoteIdent(cfg.LogsTable), cfg.Compression.Algorithm)
}

func parseInsertSpanSQL(cfg *Config) string {
	return fmt.Sprintf(insertSpanSQL, quoteIdent(cfg.Keyspace), quoteIdent(cfg.TraceTable))
}

func parseInsertLogSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogTableSQL, quoteIdent(cfg.Keyspace), quoteIdent(cfg.LogsTable))
}

func execSchema(ctx context.Context, cfg *Config, statements []string) error {
	cluster, err := newCluster(cfg)
	if err != nil {
		return err
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()

	for _, stmt := range statements {
		if err := session.Query(stmt).ExecContext(ctx); err != nil {
			return fmt.Errorf("cassandra schema: %w", err)
		}
	}
	return nil
}

func initializeTraceKernel(ctx context.Context, cfg *Config) error {
	return execSchema(ctx, cfg, []string{
		parseCreateDatabaseSQL(cfg),
		parseCreateEventsTypeSQL(cfg),
		parseCreateLinksTypeSQL(cfg),
		parseCreateSpanTableSQL(cfg),
	})
}

func initializeLogKernel(ctx context.Context, cfg *Config) error {
	return execSchema(ctx, cfg, []string{
		parseCreateDatabaseSQL(cfg),
		parseCreateLogTableSQL(cfg),
	})
}
