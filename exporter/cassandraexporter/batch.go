// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"fmt"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

// queryExecutor writes rows that share one partition key.
type queryExecutor interface {
	Execute(ctx context.Context, stmt string, rows [][]any) error
}

type insertRow struct {
	partition string
	args      []any
}

type gocqlExecutor struct {
	session *gocql.Session
}

func (g gocqlExecutor) Execute(ctx context.Context, stmt string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) == 1 {
		return g.session.Query(stmt, rows[0]...).ExecContext(ctx)
	}
	batch := g.session.Batch(gocql.UnloggedBatch)
	for _, args := range rows {
		batch.Query(stmt, args...)
	}
	return batch.ExecContext(ctx)
}

// writeRows groups rows by partition and sends unlogged batches.
// A batch that spans partitions is a Cassandra anti-pattern, so each call stays inside one partition.
func writeRows(ctx context.Context, exec queryExecutor, stmt string, rows []insertRow) error {
	if len(rows) == 0 {
		return nil
	}
	if exec == nil {
		return fmt.Errorf("cassandra exporter is not started")
	}

	grouped := make(map[string][][]any)
	order := make([]string, 0)
	for _, row := range rows {
		if _, ok := grouped[row.partition]; !ok {
			order = append(order, row.partition)
		}
		grouped[row.partition] = append(grouped[row.partition], row.args)
	}

	for _, key := range order {
		args := grouped[key]
		for start := 0; start < len(args); start += maxStatementsPerBatch {
			end := start + maxStatementsPerBatch
			if end > len(args) {
				end = len(args)
			}
			if err := exec.Execute(ctx, stmt, args[start:end]); err != nil {
				return fmt.Errorf("insert into cassandra: %w", err)
			}
		}
	}
	return nil
}
