// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	includeDatabaseParam = "includeDatabase"
	excludeDatabaseParam = "excludeDatabase"
)

// databaseFilterPlaceholder matches the {database_filter <keyword> <expression>}
// placeholder a query template puts where the database filter belongs: keyword
// joins the filter condition to the query, and expression gives the database
// name of a row.
var databaseFilterPlaceholder = regexp.MustCompile(`\{database_filter (WHERE|AND) ([^}]+)\}`)

// databaseFilter decides which databases are reported on, from the databases and
// exclude_databases settings. Names match case-insensitively, as they do under
// the default SQL Server collation.
type databaseFilter struct {
	include []string
	exclude []string
}

func newDatabaseFilter(cfg *Config) databaseFilter {
	return databaseFilter{
		include: lowerAll(cfg.Databases),
		exclude: lowerAll(cfg.ExcludeDatabases),
	}
}

func lowerAll(names []string) []string {
	lowered := make([]string, len(names))
	for i, name := range names {
		lowered[i] = strings.ToLower(name)
	}
	return lowered
}

// allows reports whether rows of the named database are reported on.
func (f databaseFilter) allows(database string) bool {
	database = strings.ToLower(database)
	if slices.Contains(f.exclude, database) {
		return false
	}
	return len(f.include) == 0 || slices.Contains(f.include, database)
}

// apply resolves the database filter placeholders of query into conditions that
// keep only the rows of allowed databases, so that the filter applies before
// TOP limits the rows. The names are bound to the parameters args returns. When
// every database is allowed the placeholders are removed.
func (f databaseFilter) apply(query string) string {
	return databaseFilterPlaceholder.ReplaceAllStringFunc(query, func(placeholder string) string {
		match := databaseFilterPlaceholder.FindStringSubmatch(placeholder)
		condition := f.condition(match[2])
		if condition == "" {
			return ""
		}
		return " " + match[1] + " " + condition
	})
}

func (f databaseFilter) condition(expression string) string {
	// A row whose database cannot be resolved has a NULL name, which NOT IN would
	// drop as well, so it is compared as an empty name: kept by exclude_databases,
	// dropped by databases.
	name := "LOWER(ISNULL(" + expression + ", N''))"
	var conditions []string
	if len(f.include) > 0 {
		conditions = append(conditions, name+" IN ("+parameterList(includeDatabaseParam, len(f.include))+")")
	}
	if len(f.exclude) > 0 {
		conditions = append(conditions, name+" NOT IN ("+parameterList(excludeDatabaseParam, len(f.exclude))+")")
	}
	return strings.Join(conditions, " AND ")
}

func parameterList(prefix string, count int) string {
	params := make([]string, count)
	for i := range params {
		params[i] = fmt.Sprintf("@%s%d", prefix, i)
	}
	return strings.Join(params, ", ")
}

// args binds the database names to the parameters of the conditions apply
// writes. It is empty when every database is allowed.
func (f databaseFilter) args() []any {
	args := make([]any, 0, len(f.include)+len(f.exclude))
	for i, name := range f.include {
		args = append(args, sql.Named(fmt.Sprintf("%s%d", includeDatabaseParam, i), name))
	}
	for i, name := range f.exclude {
		args = append(args, sql.Named(fmt.Sprintf("%s%d", excludeDatabaseParam, i), name))
	}
	return args
}
