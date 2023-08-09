// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"time"

	"cloud.google.com/go/spanner"
)

const (
	topMetricsQueryLimitParameterName = "topMetricsQueryMaxRows"
	topMetricsQueryLimitCondition     = " LIMIT @" + topMetricsQueryLimitParameterName

	pullTimestampParameterName = "pullTimestamp"
)

type statementArgs struct {
	query                  string
	topMetricsQueryMaxRows int
	pullTimestamp          time.Time
	stalenessRead          bool
}

type statsStatement struct {
	statement     spanner.Statement
	stalenessRead bool
}

func currentStatsStatement(args statementArgs) statsStatement {
	stmt := spanner.Statement{SQL: args.query, Params: map[string]interface{}{}}

	if args.topMetricsQueryMaxRows > 0 {
		stmt = spanner.Statement{
			SQL: args.query + topMetricsQueryLimitCondition,
			Params: map[string]interface{}{
				topMetricsQueryLimitParameterName: args.topMetricsQueryMaxRows,
			},
		}
	}

	return statsStatement{
		statement:     stmt,
		stalenessRead: args.stalenessRead,
	}
}

func intervalStatsStatement(args statementArgs) statsStatement {
	stmt := currentStatsStatement(args)

	if len(stmt.statement.Params) == 0 {
		stmt.statement.Params = map[string]interface{}{}
	}

	stmt.statement.Params[pullTimestampParameterName] = args.pullTimestamp

	return stmt
}
