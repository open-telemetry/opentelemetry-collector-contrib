// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import (
	"context"
	postgresqlparser "github.com/auxten/postgresql-parser/pkg/sql/parser"
	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	postgresqlwalk "github.com/auxten/postgresql-parser/pkg/walk"
	"github.com/cespare/xxhash/v2"
	cache2 "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/cache"
	mysqlparser "github.com/xwb1989/sqlparser"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
	"regexp"
	"strconv"
	"strings"
)

var DBTypes = []string{MySQL, PostgreSQL}

const (
	MySQL      = "mysql"
	PostgreSQL = "postgresql"
)

type coralogixProcessor struct {
	config *Config
	component.StartFunc
	component.ShutdownFunc
	cache  cache2.Cache[string]
	logger *zap.Logger
}

func isDBSystemInDBTypes(dbSystem string) bool {
	for _, dbType := range DBTypes {
		if dbType == dbSystem {
			return true
		}
	}
	return false
}

func fromConfigOrDefault(configValue int64, defaultValue int64) int64 {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}

func mysqlReplaceValuesWithPlaceholder(stmt mysqlparser.Statement) mysqlparser.Statement {
	err := mysqlparser.Walk(func(node mysqlparser.SQLNode) (kontinue bool, err error) {
		switch n := node.(type) {

		case *mysqlparser.Insert:
			for i, expr := range n.Rows.(mysqlparser.Values) {
				for j, val := range expr {
					if v, ok := val.(*mysqlparser.ColName); ok {
						v.Name = mysqlparser.NewColIdent("?")
					}
					expr[j] = val
				}
				n.Rows.(mysqlparser.Values)[i] = expr
			}
		case *mysqlparser.SQLVal:
			n.Type = mysqlparser.ValArg
			n.Val = []byte("?")

		case *mysqlparser.ComparisonExpr:
			if n.Operator == mysqlparser.InStr || n.Operator == mysqlparser.NotInStr {
				if v, ok := n.Right.(mysqlparser.ValTuple); ok {
					v = v[0:1]
					n.Right = v
				}
			}
		}
		return true, nil
	}, stmt)
	if err != nil {
		return nil
	}
	return stmt
}

func (sp *coralogixProcessor) postgresqlParse(dbStatementStr *string) (string, error) {
	replaceDollarInsideValues(dbStatementStr)
	stmts, err := postgresqlparser.Parse(*dbStatementStr)
	if err != nil {
		sp.logger.Error("error parsing sql", zap.Error(err))
		return "", err
	}
	w := &postgresqlwalk.AstWalker{
		Fn: func(_ any, node any) (stop bool) {
			if n, ok := node.(*tree.ComparisonExpr); ok {
				_, leftIsColumn := n.Left.(*tree.ColumnItem)
				_, leftIsUnresolved := n.Left.(*tree.UnresolvedName)
				_, rightIsColumn := n.Right.(*tree.ColumnItem)
				_, rightIsUnresolved := n.Right.(*tree.UnresolvedName)
				if leftIsColumn && !rightIsColumn || leftIsUnresolved && !rightIsUnresolved {
					n.Right = tree.NewStrVal("?")
				}
				if !leftIsColumn && rightIsColumn || !leftIsUnresolved && rightIsUnresolved {
					n.Left = tree.NewStrVal("?")
				}
			}
			return false
		},
	}
	_, _ = w.Walk(stmts, nil)
	blueprintStr := stmts.String()
	return blueprintStr, nil
}

func replaceDollarInsideValues(input *string) {
	if strings.Contains(*input, "$") {
		re := regexp.MustCompile(`(?i)VALUES\s*\(\s*([^)]+)\)`)
		// some libraries implement $1 as a placeholder, for example
		*input = re.ReplaceAllStringFunc(*input, func(match string) string {
			// Replace '$' only inside the matched "VALUES" expression
			return strings.ReplaceAll(match, "$", "")
		})
	}
}

func (sp *coralogixProcessor) mysqlParse(dbStatementStr *string) (string, error) {
	replaceDollarInsideValues(dbStatementStr)
	stmt, err := mysqlparser.Parse(*dbStatementStr)
	if err != nil {
		sp.logger.Error("error parsing sql", zap.Error(err))
		return "", err
	}
	blueprint := mysqlReplaceValuesWithPlaceholder(stmt)
	blueprintStr := mysqlparser.String(blueprint)
	return blueprintStr, nil
}

func (sp *coralogixProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				attributes := span.Attributes()
				dbStatement, dbStatementExists := attributes.Get("db.statement")
				if !dbStatementExists {
					return td, nil
				}
				dbSystem, dbSystemExists := attributes.Get("db.system")
				if !dbSystemExists {
					return td, nil
				}
				dbSystemStr := dbSystem.AsString()
				if !isDBSystemInDBTypes(dbSystemStr) {
					return td, nil
				}
				dbStatementStr := dbStatement.AsString()
				var blueprintStr string
				var err error
				if dbSystemStr == MySQL {
					blueprintStr, err = sp.mysqlParse(&dbStatementStr)
					if err != nil {
						return ptrace.Traces{}, err
					}
				}
				if dbSystemStr == PostgreSQL {
					blueprintStr, err = sp.postgresqlParse(&dbStatementStr)
					if err != nil {
						return ptrace.Traces{}, err
					}
				}

				hash := xxhash.Sum64String(blueprintStr)
				if sp.config.sampling.enabled {
					_, found := sp.cache.Get(hash)
					if !found {
						attributes.PutInt("sampling.priority", 100)
						sp.cache.Add(hash, "")
					}
				}
				attributes.PutStr("db.statement.blueprint", blueprintStr)
				attributes.PutStr("db.statement.blueprint.id", strconv.FormatUint(hash, 16))
			}
		}
	}
	return td, nil
}

func newCoralogixProcessor(ctx context.Context, set processor.Settings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	sp := &coralogixProcessor{
		config: cfg,
	}

	if cfg.databaseBlueprintsConfig.sampling.enabled {

		// 8 bytes per entry, 1GB / 8 Bytes = 134,217,728 entries by default
		var cacheSize = fromConfigOrDefault(cfg.databaseBlueprintsConfig.sampling.maxCacheSizeMib*1024*1024, 1<<30) / 8 // Default to 1GB
		var err error
		sp.cache, err = cache2.NewLRUBlueprintCache[string](int(cacheSize))
		if err != nil {
			return nil, err
		}
	}

	return processorhelper.NewTracesProcessor(ctx,
		set,
		cfg,
		nextConsumer,
		sp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
