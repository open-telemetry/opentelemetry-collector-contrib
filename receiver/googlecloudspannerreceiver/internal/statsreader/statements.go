// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
