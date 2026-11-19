// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

type explainSetupState struct {
	available bool
	expiresAt time.Time
}

// quoteExplainFunctionName quotes each segment of a validated [schema.]name.
func quoteExplainFunctionName(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, ".")
}

func (c *postgreSQLClient) probeExplainFunction(ctx context.Context, quotedFunctionName string) error {
	query := fmt.Sprintf("/* otel-collector-ignore */ SELECT %s('SELECT 1')", quotedFunctionName) // #nosec G201 -- function name is validated and quoted
	var plan string
	return c.client.QueryRowContext(ctx, query).Scan(&plan)
}

// explainFunction caches both successful and failed probes per database.
func (p *postgreSQLScraper) explainFunction(ctx context.Context, database string, dbClient client) string {
	name := p.config.TopQueryCollection.ExplainFunctionName
	if name == "" {
		return ""
	}
	quoted := quoteExplainFunctionName(name)
	state, cached := p.explainFunctionCache.Get(database)
	if !cached || !time.Now().Before(state.expiresAt) {
		err := dbClient.probeExplainFunction(ctx, quoted)
		state = explainSetupState{available: err == nil, expiresAt: time.Now().Add(p.config.TopQueryCollection.ExplainFunctionCacheTTL)}
		p.explainFunctionCache.Add(database, state)
		if err != nil {
			p.logger.Warn("EXPLAIN helper function unavailable, falling back to inline EXPLAIN",
				zap.String("database", database), zap.String("explain_function_name", name), zap.Error(err))
		}
	}
	if !state.available {
		return ""
	}
	return quoted
}

func (c *postgreSQLClient) explainQueryViaFunction(ctx context.Context, query, queryID, explainFunction string, logger *zap.Logger) (string, error) {
	statement := fmt.Sprintf("/* otel-collector-ignore */ SELECT %s($1) AS query_plan", explainFunction) // #nosec G201 -- function name is validated and quoted
	db := sqlquery.NewDbClient(sqlquery.DbWrapper{Db: c.client}, statement, logger, sqlquery.TelemetryConfig{})
	result, err := db.QueryRows(ctx, query)
	if err != nil {
		return "", err
	}
	if len(result) == 0 {
		return "", nil
	}
	plan, err := obfuscateSQLExecPlan(result[0]["query_plan"])
	if err != nil {
		logger.Error("failed to obfuscate explain plan", zap.Error(err), zap.String("queryID", queryID))
	}
	return plan, err
}
