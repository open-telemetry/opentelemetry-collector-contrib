// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

const querySampleTraceContextKey = "_otel_trace_context"

// detachedCleanupTimeout bounds the best-effort DEALLOCATE PREPARE cleanup in
// explainQuery so a truly unresponsive backend can't hang cleanup forever.
const detachedCleanupTimeout = 5 * time.Second

// databaseName is a name that refers to a database so that it can be uniquely referred to later
// i.e. database1
type databaseName string

// tableIdentifier is an identifier that contains both the database and table separated by a "|"
// i.e. database1|table2
type tableIdentifier string

// indexIdentifier is a unique string that identifies a particular index and is separated by the "|" character
type indexIdentifer string

// functionIdentifier is a unique string that identifies a particular function and is separated by the "|" character
type functionIdentifer string

// errNoLastArchive is an error that occurs when there is no previous wal archive, so there is no way to compute the
// last archived point
var errNoLastArchive = errors.New("no last archive found, not able to calculate oldest WAL age")

type client interface {
	Close() error
	getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error)
	getExecutionTimeStats(ctx context.Context, databases []string) (map[databaseName]float64, error)
	getDatabaseConflicts(ctx context.Context, databases []string) (map[databaseName]databaseConflictStats, error)
	getDatabaseLocks(ctx context.Context) ([]databaseLocks, error)
	getServerScopedLocks(ctx context.Context) ([]databaseLocks, error)
	getBGWriterStats(ctx context.Context) (*bgStat, error)
	getBackends(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseSize(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error)
	getBlocksReadByTable(ctx context.Context, db string) (map[tableIdentifier]tableIOStats, error)
	getTableCount(ctx context.Context) (int64, error)
	getReplicationStats(ctx context.Context) ([]replicationStats, error)
	getLatestWalAgeSeconds(ctx context.Context) (int64, error)
	getMaxConnections(ctx context.Context) (int64, error)
	getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error)
	getFunctionStats(ctx context.Context, database string) (map[functionIdentifer]functionStat, error)
	getVectorSearchStats(ctx context.Context) ([]vectorSearchStat, error)
	getVectorInsertStats(ctx context.Context) ([]vectorInsertStat, error)
	listDatabases(ctx context.Context) ([]string, error)
	getVersion(ctx context.Context) (string, error)
	getQuerySamples(ctx context.Context, limit int64, newestQueryTimestamp float64, excludedDatabases []string, logger *zap.Logger) ([]map[string]any, float64, error)
	getTopQuery(ctx context.Context, limit int64, excludedDatabases []string, logger *zap.Logger) ([]map[string]any, error)
	explainQuery(ctx context.Context, query, queryID string, logger *zap.Logger) (string, error)
}

type postgreSQLClient struct {
	client  *sql.DB
	closeFn func() error
}

// explainableStatements is a whitelist of SQL statements that PostgreSQL can EXPLAIN.
var explainableStatements = map[string]struct{}{
	"SELECT": {},
	"TABLE":  {}, // TABLE is shorthand for SELECT * FROM
	"DELETE": {},
	"INSERT": {},
	"UPDATE": {},
	"WITH":   {}, // CTEs
	"MERGE":  {}, // PostgreSQL 15+
	"VALUES": {},
}

// isExplainableQuery checks if a query can be explained by PostgreSQL.
// Uses a whitelist approach, only allows known DML statements.
func isExplainableQuery(query string) bool {
	trimmed := strings.TrimSpace(query)

	// Remove leading comments (both -- and /* */ style)
	for {
		switch {
		case strings.HasPrefix(trimmed, "--"):
			idx := strings.Index(trimmed, "\n")
			if idx == -1 {
				return false
			}
			trimmed = strings.TrimSpace(trimmed[idx+1:])
			continue
		case strings.HasPrefix(trimmed, "/*"):
			idx := strings.Index(trimmed, "*/")
			if idx == -1 {
				return false
			}
			trimmed = strings.TrimSpace(trimmed[idx+2:])
			continue
		}
		break
	}

	if trimmed == "" {
		return false
	}

	// Extract and uppercase only the first word to check against the whitelist
	firstWord := trimmed
	if idx := strings.IndexAny(trimmed, " \t\n("); idx != -1 {
		firstWord = trimmed[:idx]
	}

	_, ok := explainableStatements[strings.ToUpper(firstWord)]
	return ok
}

// typedLiteralTypeNames is the allowlist of type names repaired in the "TYPENAME $N" position,
// longest spelling first so a multi-word name beats the prefix it starts with. Anything unlisted,
// including aliases such as int and bool and any user-defined type, stays unrepaired.
const typedLiteralTypeNames = `timestamptz|timestamp\s+with\s+time\s+zone|timestamp\s+without\s+time\s+zone|timestamp|` +
	`timetz|time\s+with\s+time\s+zone|time\s+without\s+time\s+zone|time|interval|date|` +
	`double\s+precision|numeric|decimal|real|smallint|integer|bigint|boolean|` +
	`uuid|jsonb|json|xml|bytea|inet|cidr|macaddr|money|bit\s+varying|bit|` +
	`character\s+varying|character|varchar|text`

// pg_stat_statements substitutes "$N" over the byte ranges of the constants it jumbled without
// re-parsing, so a parameter can land where the grammar accepts none. Both constructs below fail
// PREPARE with 42601; both rewrites skip protectedSpans, so a "$N" that is only text is untouched.
var (
	// EXTRACT's field argument cannot be a parameter, so the emitted EXTRACT($1 FROM x) is
	// invalid; pg_catalog.extract($1, x) is the call the parser itself builds and does take one.
	// Only the text through FROM is replaced, so the original closing paren still terminates the
	// call. EXTRACT($2 FROM $1) is rewritten but fails with 42725: untyped args pick no overload.
	extractParamPattern = regexp.MustCompile(`(?i)\bEXTRACT\s*\(\s*(\$\d+)\s+FROM\s+`)

	// TYPENAME 'value' requires a literal, so "interval '1 day'" normalizes to the invalid
	// "interval $1". CAST is used rather than "::" because only it takes a multi-word type name or
	// an interval qualifier; the trailing group accepts only interval keywords so it cannot swallow
	// a following AND, OR or alias. Required whitespace leaves "interval_col" and "timestamp(3) $1".
	typedLiteralParamPattern = regexp.MustCompile(`(?i)\b(` + typedLiteralTypeNames + `)\s+(\$\d+)\b` +
		`((?:\s+(?:YEAR|MONTH|DAY|HOUR|MINUTE|SECOND)(?:\s+TO\s+(?:MONTH|DAY|HOUR|MINUTE|SECOND))?)?)`)
)

// repairNormalizedQuery rewrites the parameter placements that pg_stat_statements can emit but
// PostgreSQL cannot parse. Queries that do not contain them are returned unchanged.
//
// The EXTRACT repair targets pg_catalog.extract (PostgreSQL 14+) rather than date_part, whose
// double precision return would fail to prepare wherever the result feeds a numeric-only function
// or operator such as round(x, n) or %. No version gate is needed: before 14 EXTRACT returned
// double precision, so such a query would not have parsed and cannot have been recorded.
func repairNormalizedQuery(query string) string {
	query = replaceOutsideProtected(query, extractParamPattern, "pg_catalog.extract(${1}, ")
	query = replaceOutsideProtected(query, typedLiteralParamPattern, "CAST(${2} AS ${1}${3})")
	return query
}

// replaceOutsideProtected applies re the way ReplaceAllString would, except that a match beginning
// inside a protectedSpans range is left alone. Spans are recomputed per call.
func replaceOutsideProtected(query string, re *regexp.Regexp, repl string) string {
	matches := re.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query
	}

	spans := protectedSpans(query)
	var out []byte
	last := 0
	for _, m := range matches {
		if isProtectedOffset(spans, m[0]) {
			continue
		}
		out = append(out, query[last:m[0]]...)
		out = re.ExpandString(out, repl, query, m)
		last = m[1]
	}
	if out == nil {
		return query
	}
	return string(append(out, query[last:]...))
}

// isProtectedOffset reports whether offset falls inside one of the spans, which protectedSpans
// returns ordered and non-overlapping.
func isProtectedOffset(spans [][2]int, offset int) bool {
	for _, s := range spans {
		if offset < s[0] {
			return false
		}
		if offset < s[1] {
			return true
		}
	}
	return false
}

// protectedSpans reports the byte ranges a repair must not rewrite: string literals, quoted
// identifiers, dollar-quoted strings and comments, which pg_stat_statements preserves verbatim, so
// a "$1" inside one is text. Unterminated constructs protect to the end; over-protecting only
// skips a repair, while under-protecting corrupts SQL that prepares today.
func protectedSpans(query string) [][2]int {
	var spans [][2]int
	for i := 0; i < len(query); {
		end, ok := protectedSpanAt(query, i)
		if !ok {
			i++
			continue
		}
		spans = append(spans, [2]int{i, end})
		i = end
	}
	return spans
}

// protectedSpanAt reports the end of the protected region starting at i, or false when nothing
// protected starts there.
func protectedSpanAt(query string, i int) (int, bool) {
	switch {
	case query[i] == '\'':
		return endOfQuoted(query, i, escapesWithBackslash(query, i)), true
	case query[i] == '"':
		return endOfQuoted(query, i, false), true
	case strings.HasPrefix(query[i:], "--"):
		if nl := strings.IndexByte(query[i:], '\n'); nl != -1 {
			return i + nl, true
		}
		return len(query), true
	case strings.HasPrefix(query[i:], "/*"):
		return endOfBlockComment(query, i), true
	case query[i] == '$':
		tag, ok := dollarQuoteTag(query, i)
		if !ok {
			return 0, false
		}
		return endOfDollarQuoted(query, i, tag), true
	default:
		return 0, false
	}
}

// endOfQuoted returns the index just past the quote that closes the run opening at start, or
// len(query) when it is unterminated.
func endOfQuoted(query string, start int, backslashEscapes bool) int {
	quote := query[start]
	for i := start + 1; i < len(query); {
		switch {
		case backslashEscapes && query[i] == '\\':
			i += 2
		case query[i] != quote:
			i++
		// A doubled quote is an escaped quote, so it continues the run rather than closing it.
		case i+1 < len(query) && query[i+1] == quote:
			i += 2
		default:
			return i + 1
		}
	}
	return len(query)
}

// escapesWithBackslash reports whether the literal opening at start is an E'...' string, the only
// form where a backslash escapes the next byte. The E must not be the tail of a longer word.
func escapesWithBackslash(query string, start int) bool {
	if start == 0 || (query[start-1] != 'E' && query[start-1] != 'e') {
		return false
	}
	return start < 2 || !isIdentifierByte(query[start-2])
}

// endOfBlockComment returns the index just past the "*/" that closes the comment opening at
// start, or len(query) when it is unterminated. Block comments nest, hence the depth counter.
func endOfBlockComment(query string, start int) int {
	depth := 0
	for i := start; i+1 < len(query); {
		switch query[i : i+2] {
		case "/*":
			depth++
			i += 2
		case "*/":
			depth--
			if depth == 0 {
				return i + 2
			}
			i += 2
		default:
			i++
		}
	}
	return len(query)
}

// dollarQuoteTag returns the full delimiter, both dollar signs included, of the dollar-quoted
// string at start. A tag is empty or an identifier not starting with a digit, so "$1" is not one.
func dollarQuoteTag(query string, start int) (string, bool) {
	for i := start + 1; i < len(query); i++ {
		if query[i] == '$' {
			return query[start : i+1], true
		}
		if !isIdentifierByte(query[i]) || (i == start+1 && query[i] >= '0' && query[i] <= '9') {
			return "", false
		}
	}
	return "", false
}

// endOfDollarQuoted returns the index just past the closing tag of the dollar-quoted string
// opening at start, or len(query) when it is unterminated.
func endOfDollarQuoted(query string, start int, tag string) int {
	body := start + len(tag)
	if offset := strings.Index(query[body:], tag); offset != -1 {
		return body + offset + len(tag)
	}
	return len(query)
}

func isIdentifierByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// explainQuery implements client.
//
// The real parameter count comes from pg_prepared_statements, not from counting "$N"
// occurrences in the query text: a placeholder can repeat (e.g. "$1" used twice for one
// real parameter) or appear inside a string literal ("$1" is not a placeholder there), and
// either case makes a text-based count wrong. Once PREPARE succeeds, PostgreSQL has already
// parsed and deduplicated the real list, so pg_prepared_statements.parameter_types is
// authoritative. Reading it back requires a second round trip after PREPARE, and since
// PREPARE is session-scoped, every step (PREPARE, the count lookup, EXPLAIN EXECUTE,
// DEALLOCATE) has to run on the one connection that ran PREPARE, not just whatever the pool
// hands out for each call.
func (c *postgreSQLClient) explainQuery(ctx context.Context, query, queryID string, logger *zap.Logger) (string, error) {
	// Check if the query is explainable before attempting EXPLAIN
	if !isExplainableQuery(query) {
		logger.Debug("skipping EXPLAIN for non-explainable query", zap.String("queryID", queryID))
		return "", nil
	}

	if repaired := repairNormalizedQuery(query); repaired != query {
		logger.Debug("repaired normalized query before EXPLAIN",
			zap.String("queryID", queryID),
			zap.String("normalizedQuery", query),
			zap.String("preparedQuery", repaired))
		query = repaired
	}

	normalizedQueryID := strings.ReplaceAll(queryID, "-", "_")

	conn, err := c.client.Conn(ctx)
	if err != nil {
		logger.Error("failed to obtain a dedicated connection for EXPLAIN", zap.Error(err), zap.String("queryID", queryID))
		return "", err
	}
	defer conn.Close()

	// Deallocate the prepared statement on a context detached from ctx so the
	// cleanup still runs even if ctx was already canceled or timed out. Runs on the
	// same dedicated connection that ran PREPARE, since DEALLOCATE on any other
	// connection wouldn't find it.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachedCleanupTimeout)
		defer cancel()
		_, _ = conn.ExecContext(cleanupCtx, fmt.Sprintf("/* otel-collector-ignore */ DEALLOCATE PREPARE otel_%s", normalizedQueryID))
	}()

	setPlanCacheMode := "/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;"
	prepareStatement := fmt.Sprintf("PREPARE otel_%s AS %s;", normalizedQueryID, query)

	prepareDb := sqlquery.NewDbClient(sqlquery.ConnWrapper{Conn: conn}, setPlanCacheMode+prepareStatement, logger, sqlquery.TelemetryConfig{})
	if _, err = prepareDb.QueryRows(ctx); err != nil {
		// A syntax error's character position refers to the repaired text, not the caller's.
		logger.Error("failed to prepare statement for EXPLAIN", zap.Error(err), zap.String("queryID", queryID), zap.String("preparedQuery", query))
		return "", err
	}

	paramCountSQL := fmt.Sprintf(
		"/* otel-collector-ignore */ SELECT COALESCE(array_length(parameter_types, 1), 0) AS param_count FROM pg_prepared_statements WHERE name = 'otel_%s';",
		normalizedQueryID,
	)
	paramCountDb := sqlquery.NewDbClient(sqlquery.ConnWrapper{Conn: conn}, paramCountSQL, logger, sqlquery.TelemetryConfig{})
	paramCountResult, err := paramCountDb.QueryRows(ctx)
	if err != nil {
		logger.Error("failed to look up prepared statement parameter count", zap.Error(err), zap.String("queryID", queryID))
		return "", err
	}
	if len(paramCountResult) == 0 {
		logger.Error("prepared statement not found in pg_prepared_statements after PREPARE succeeded", zap.String("queryID", queryID))
		return "", fmt.Errorf("prepared statement otel_%s not found in pg_prepared_statements", normalizedQueryID)
	}

	paramCount, err := strconv.Atoi(paramCountResult[0]["param_count"])
	if err != nil {
		logger.Error("failed to parse prepared statement parameter count", zap.Error(err), zap.String("queryID", queryID))
		return "", err
	}

	nullsString := ""
	if paramCount > 0 {
		nulls := make([]string, paramCount)
		for i := range nulls {
			nulls[i] = "null"
		}
		nullsString = "(" + strings.Join(nulls, ", ") + ")"
	}
	explainStatement := fmt.Sprintf("EXPLAIN(FORMAT JSON) EXECUTE otel_%s%s;", normalizedQueryID, nullsString)

	explainDb := sqlquery.NewDbClient(sqlquery.ConnWrapper{Conn: conn}, explainStatement, logger, sqlquery.TelemetryConfig{})
	result, err := explainDb.QueryRows(ctx)
	if err != nil {
		logger.Error("failed to explain statement", zap.Error(err))
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	plan, err := obfuscateSQLExecPlan(result[0]["QUERY PLAN"])
	if err != nil {
		logger.Error("failed to obfuscate explain plan", zap.Error(err), zap.String("queryID", queryID))
		return "", err
	}

	return plan, nil
}

var _ client = (*postgreSQLClient)(nil)

type postgreSQLConfig struct {
	username string
	password string
	database string
	address  confignet.AddrConfig
	tls      configtls.ClientConfig
	// credentialProvider, when non-nil, supplies the password (and optionally the
	// username) at connection-string-build time instead of the static password.
	// Non-pool path: resolved once per *sql.DB build (one build per scrape), so
	// each scrape picks up a freshly-minted credential. Pool path: resolved per
	// physical connection by credentialConnector, so a long-lived pool re-mints on
	// every new connection it opens. Either way, no collector restart is needed.
	credentialProvider dbauth.Provider
}

var conninfoValueEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// quoteConninfoValue encodes a value for lib/pq's keyword/value connection
// string format. Quoting every value prevents credential or configuration data
// containing whitespace, quotes, or backslashes from becoming new options.
func quoteConninfoValue(value string) string {
	return `'` + conninfoValueEscaper.Replace(value) + `'`
}

func sslConnectionString(tls configtls.ClientConfig) string {
	if tls.Insecure {
		return "sslmode='disable'"
	}

	conn := ""

	if tls.InsecureSkipVerify {
		conn += "sslmode='require'"
	} else {
		conn += "sslmode='verify-full'"
	}

	if tls.CAFile != "" {
		conn += " sslrootcert=" + quoteConninfoValue(tls.CAFile)
	}

	if tls.KeyFile != "" {
		conn += " sslkey=" + quoteConninfoValue(tls.KeyFile)
	}

	if tls.CertFile != "" {
		conn += " sslcert=" + quoteConninfoValue(tls.CertFile)
	}

	return conn
}

// ConnectionString builds the lib/pq DSN, resolving the credential provider (if
// any) with the supplied context. The pool path resolves per physical connection
// via credentialConnector, so the connection context flows through to credential
// minting.
func (c postgreSQLConfig) ConnectionString(ctx context.Context) (string, error) {
	// postgres will assume the supplied user as the database name if none is provided,
	// so we must specify a database name even when we are just collecting the list of databases.
	database := defaultPostgreSQLDatabase
	if c.database != "" {
		database = c.database
	}

	host, port, err := net.SplitHostPort(c.address.Endpoint)
	if err != nil {
		return "", err
	}

	if c.address.Transport == confignet.TransportTypeUnix {
		// lib/pg expects a unix socket host to start with a "/" and appends the appropriate .s.PGSQL.port internally
		host = "/" + host
	}

	username, password := c.username, c.password
	if c.credentialProvider != nil {
		// Resolve the credential at build time so each new connection uses a
		// currently-valid secret (e.g. a freshly-minted AWS IAM token).
		cred, credErr := c.credentialProvider.GetCredential(ctx, dbauth.Request{
			Endpoint: c.address.Endpoint,
			Username: c.username,
		})
		if credErr != nil {
			return "", fmt.Errorf("resolve credential: %w", credErr)
		}
		if cred == nil {
			// A provider must return either a credential or an error. Guard against a
			// contract-violating provider so a bad extension fails this scrape closed
			// rather than panicking the whole collector on the nil dereference below.
			return "", errors.New("resolve credential: provider returned a nil credential")
		}
		password = cred.Secret
		if cred.Username != nil {
			username = *cred.Username
		}
	}

	return fmt.Sprintf(
		"port=%s host=%s user=%s password=%s dbname=%s %s",
		quoteConninfoValue(port),
		quoteConninfoValue(host),
		quoteConninfoValue(username),
		quoteConninfoValue(password),
		quoteConninfoValue(database),
		sslConnectionString(c.tls),
	), nil
}

func (c *postgreSQLClient) Close() error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

type databaseStats struct {
	transactionCommitted int64
	transactionRollback  int64
	deadlocks            int64
	tempIo               int64
	tempFiles            int64
	tupUpdated           int64
	tupReturned          int64
	tupFetched           int64
	tupInserted          int64
	tupDeleted           int64
	blksHit              int64
	blksRead             int64
}

func (c *postgreSQLClient) getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error) {
	query := filterQueryByDatabases(
		"SELECT datname, xact_commit, xact_rollback, deadlocks, temp_files, temp_bytes, tup_updated, tup_returned, tup_fetched, tup_inserted, tup_deleted, blks_hit, blks_read FROM pg_stat_database",
		databases,
		false,
	)

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var errs error
	dbStats := map[databaseName]databaseStats{}

	for rows.Next() {
		var datname string
		var transactionCommitted, transactionRollback, deadlocks, tempIo, tempFiles, tupUpdated, tupReturned, tupFetched, tupInserted, tupDeleted, blksHit, blksRead int64
		err = rows.Scan(&datname, &transactionCommitted, &transactionRollback, &deadlocks, &tempFiles, &tempIo, &tupUpdated, &tupReturned, &tupFetched, &tupInserted, &tupDeleted, &blksHit, &blksRead)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		if datname != "" {
			dbStats[databaseName(datname)] = databaseStats{
				transactionCommitted: transactionCommitted,
				transactionRollback:  transactionRollback,
				deadlocks:            deadlocks,
				tempIo:               tempIo,
				tempFiles:            tempFiles,
				tupUpdated:           tupUpdated,
				tupReturned:          tupReturned,
				tupFetched:           tupFetched,
				tupInserted:          tupInserted,
				tupDeleted:           tupDeleted,
				blksHit:              blksHit,
				blksRead:             blksRead,
			}
		}
	}
	return dbStats, errs
}

// getExecutionTimeStats returns, per database, the cumulative time (in seconds) spent executing
// SQL statements. It aggregates the total_exec_time column of pg_stat_statements (reported in
// milliseconds) across all currently tracked statements and requires the pg_stat_statements
// extension to be installed.
func (c *postgreSQLClient) getExecutionTimeStats(ctx context.Context, databases []string) (map[databaseName]float64, error) {
	query := filterQueryByDatabases(
		"SELECT pd.datname AS datname, SUM(pss.total_exec_time) / 1000.0 AS execution_time_seconds FROM pg_stat_statements pss JOIN pg_database pd ON pss.dbid = pd.oid",
		databases,
		true,
	)

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errs error
	stats := map[databaseName]float64{}

	for rows.Next() {
		var datname string
		var executionTime float64
		err = rows.Scan(&datname, &executionTime)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		if datname != "" {
			stats[databaseName(datname)] = executionTime
		}
	}
	return stats, multierr.Append(errs, rows.Err())
}

// databaseConflictStats holds the per-database query cancellation counters from
// pg_stat_database_conflicts. These counters are only incremented on standby
// servers, where queries can be canceled due to conflicts with recovery.
type databaseConflictStats struct {
	conflTablespace int64
	conflLock       int64
	conflSnapshot   int64
	conflBufferpin  int64
	conflDeadlock   int64
}

func (c *postgreSQLClient) getDatabaseConflicts(ctx context.Context, databases []string) (map[databaseName]databaseConflictStats, error) {
	query := filterQueryByDatabases(
		"SELECT datname, confl_tablespace, confl_lock, confl_snapshot, confl_bufferpin, confl_deadlock FROM pg_stat_database_conflicts",
		databases,
		false,
	)

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errs error
	conflictStats := map[databaseName]databaseConflictStats{}

	for rows.Next() {
		var datname string
		var conflTablespace, conflLock, conflSnapshot, conflBufferpin, conflDeadlock int64
		err = rows.Scan(&datname, &conflTablespace, &conflLock, &conflSnapshot, &conflBufferpin, &conflDeadlock)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		if datname != "" {
			conflictStats[databaseName(datname)] = databaseConflictStats{
				conflTablespace: conflTablespace,
				conflLock:       conflLock,
				conflSnapshot:   conflSnapshot,
				conflBufferpin:  conflBufferpin,
				conflDeadlock:   conflDeadlock,
			}
		}
	}
	return conflictStats, multierr.Append(errs, rows.Err())
}

type databaseLocks struct {
	relation string
	mode     string
	lockType string
	locks    int64
}

func (c *postgreSQLClient) getDatabaseLocks(ctx context.Context) ([]databaseLocks, error) {
	// Scoped to the connected database: relation OIDs from other databases would
	// not resolve against this pg_class, and locks owned by no database are
	// collected once by getServerScopedLocks. The outer join keeps targets that
	// are not relations, which report an empty relation.
	return c.queryDatabaseLocks(ctx, `SELECT COALESCE(relname, '') AS relation, mode, locktype,COUNT(*)
	AS locks FROM pg_locks
	LEFT JOIN pg_class ON pg_locks.relation = pg_class.oid
	WHERE pg_locks.database = (SELECT oid FROM pg_database WHERE datname = current_database())
	GROUP BY relname, mode, locktype;`)
}

func (c *postgreSQLClient) getServerScopedLocks(ctx context.Context) ([]databaseLocks, error) {
	// Locks owned by no single database, collected once per scrape: shared targets
	// (database = 0, resolvable from any connection) and transaction ID targets
	// (database IS NULL). The relation IS NULL branch is required because the outer
	// join leaves relisshared NULL for targets that are not relations.
	return c.queryDatabaseLocks(ctx, `SELECT COALESCE(relname, '') AS relation, mode, locktype,COUNT(*)
	AS locks FROM pg_locks
	LEFT JOIN pg_class ON pg_locks.relation = pg_class.oid
	WHERE pg_locks.database IS NULL
	OR (pg_locks.database = 0 AND (pg_locks.relation IS NULL OR pg_class.relisshared))
	GROUP BY relname, mode, locktype;`)
}

func (c *postgreSQLClient) queryDatabaseLocks(ctx context.Context, query string) ([]databaseLocks, error) {
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_locks: %w", err)
	}
	defer rows.Close()
	var dl []databaseLocks
	var errs []error
	for rows.Next() {
		var relation, mode, lockType string
		var locks int64
		err = rows.Scan(&relation, &mode, &lockType, &locks)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		dl = append(dl, databaseLocks{
			relation: relation,
			mode:     mode,
			lockType: lockType,
			locks:    locks,
		})
	}
	return dl, multierr.Combine(errs...)
}

// getBackends returns the number of backend processes for each database, counted from pg_stat_activity
// across all connection states (active, idle, idle-in-transaction) and all backend types, including
// non-client backends such as autovacuum and parallel workers. Backends with no associated database
// (NULL datname, e.g. the background writer and WAL writer) are not attributed to any database.
func (c *postgreSQLClient) getBackends(ctx context.Context, databases []string) (map[databaseName]int64, error) {
	query := filterQueryByDatabases("SELECT datname, count(*) as count from pg_stat_activity", databases, true)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ars := map[databaseName]int64{}
	var errors error
	for rows.Next() {
		var datname string
		var count int64
		err = rows.Scan(&datname, &count)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		if datname != "" {
			ars[databaseName(datname)] = count
		}
	}
	return ars, errors
}

func (c *postgreSQLClient) getDatabaseSize(ctx context.Context, databases []string) (map[databaseName]int64, error) {
	query := filterQueryByDatabases("SELECT datname, pg_database_size(datname) FROM pg_catalog.pg_database WHERE datistemplate = false", databases, false)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sizes := map[databaseName]int64{}
	var errors error
	for rows.Next() {
		var datname string
		var size int64
		err = rows.Scan(&datname, &size)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		if datname != "" {
			sizes[databaseName(datname)] = size
		}
	}
	return sizes, errors
}

// tableStats contains a result for a row of the getDatabaseTableMetrics result
type tableStats struct {
	database    string
	schema      string
	table       string
	live        int64
	dead        int64
	inserts     int64
	upd         int64
	del         int64
	hotUpd      int64
	seqScans    int64
	size        int64
	vacuumCount int64
}

func (c *postgreSQLClient) getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error) {
	// explicitly ignore the relations which have an active `AccessExclusiveLock`
	// this is to prevent the current query's `AccessShareLock` from getting stalled
	query := `SELECT
    s.schemaname AS schema,
    s.relname AS table,
    s.n_live_tup AS live,
    s.n_dead_tup AS dead,
    s.n_tup_ins AS ins,
    s.n_tup_upd AS upd,
    s.n_tup_del AS del,
    s.n_tup_hot_upd AS hot_upd,
    s.seq_scan AS seq_scans,
    pg_relation_size(s.relid) AS table_size,
    s.vacuum_count
FROM pg_stat_user_tables s
LEFT JOIN (
    SELECT DISTINCT relation
    FROM pg_locks
    WHERE locktype = 'relation'
      AND mode = 'AccessExclusiveLock'
      AND granted = true
) l ON s.relid = l.relation
WHERE l.relation IS NULL;`

	ts := map[tableIdentifier]tableStats{}
	var errors error
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var schema, table string
		var live, dead, ins, upd, del, hotUpd, seqScans, tableSize, vacuumCount int64
		err = rows.Scan(&schema, &table, &live, &dead, &ins, &upd, &del, &hotUpd, &seqScans, &tableSize, &vacuumCount)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		ts[tableKey(db, schema, table)] = tableStats{
			database:    db,
			schema:      schema,
			table:       table,
			live:        live,
			dead:        dead,
			inserts:     ins,
			upd:         upd,
			del:         del,
			hotUpd:      hotUpd,
			seqScans:    seqScans,
			size:        tableSize,
			vacuumCount: vacuumCount,
		}
	}
	return ts, errors
}

type tableIOStats struct {
	database  string
	schema    string
	table     string
	heapRead  int64
	heapHit   int64
	idxRead   int64
	idxHit    int64
	toastRead int64
	toastHit  int64
	tidxRead  int64
	tidxHit   int64
}

func (c *postgreSQLClient) getBlocksReadByTable(ctx context.Context, db string) (map[tableIdentifier]tableIOStats, error) {
	query := `SELECT schemaname as schema, relname AS table,
	coalesce(heap_blks_read, 0) AS heap_read,
	coalesce(heap_blks_hit, 0) AS heap_hit,
	coalesce(idx_blks_read, 0) AS idx_read,
	coalesce(idx_blks_hit, 0) AS idx_hit,
	coalesce(toast_blks_read, 0) AS toast_read,
	coalesce(toast_blks_hit, 0) AS toast_hit,
	coalesce(tidx_blks_read, 0) AS tidx_read,
	coalesce(tidx_blks_hit, 0) AS tidx_hit
	FROM pg_statio_user_tables;`

	tios := map[tableIdentifier]tableIOStats{}
	var errors error
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var schema, table string
		var heapRead, heapHit, idxRead, idxHit, toastRead, toastHit, tidxRead, tidxHit int64
		err = rows.Scan(&schema, &table, &heapRead, &heapHit, &idxRead, &idxHit, &toastRead, &toastHit, &tidxRead, &tidxHit)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		tios[tableKey(db, schema, table)] = tableIOStats{
			database:  db,
			schema:    schema,
			table:     table,
			heapRead:  heapRead,
			heapHit:   heapHit,
			idxRead:   idxRead,
			idxHit:    idxHit,
			toastRead: toastRead,
			toastHit:  toastHit,
			tidxRead:  tidxRead,
			tidxHit:   tidxHit,
		}
	}
	return tios, errors
}

// getTableCount is a cheap COUNT(*) alternative to getDatabaseTableMetrics;
// must return the same count as len(getDatabaseTableMetrics).
func (c *postgreSQLClient) getTableCount(ctx context.Context) (int64, error) {
	version, err := c.getVersion(ctx)
	if err != nil {
		return 0, err
	}
	major, err := parseMajorVersion(version)
	if err != nil {
		return 0, err
	}

	// Partitioned parents ('p') only count as user tables from PG 14 on.
	query := `SELECT count(*) FROM pg_class
WHERE relkind IN ('r', 'm')
AND relnamespace NOT IN (
    SELECT oid FROM pg_namespace
    WHERE nspname = 'pg_catalog' OR nspname = 'information_schema' OR nspname ~ '^pg_toast'
);`
	if major >= 14 {
		query = `SELECT count(*) FROM pg_class
WHERE relkind IN ('r', 'm', 'p')
AND relnamespace NOT IN (
    SELECT oid FROM pg_namespace
    WHERE nspname = 'pg_catalog' OR nspname = 'information_schema' OR nspname ~ '^pg_toast'
);`
	}

	row := c.client.QueryRowContext(ctx, query)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type indexStat struct {
	index    string
	table    string
	schema   string
	database string
	size     int64
	scans    int64
}

func (c *postgreSQLClient) getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error) {
	// explicitly ignore indexes which have an active `AccessExclusiveLock`
	// this is to prevent the current query's `AccessShareLock` from getting stalled
	query := `SELECT schemaname, relname, indexrelname,
	pg_relation_size(indexrelid) AS index_size,
	idx_scan
	FROM pg_stat_user_indexes s
	LEFT JOIN (
		SELECT DISTINCT relation
		FROM pg_locks
		WHERE locktype = 'relation'
		  AND mode = 'AccessExclusiveLock'
		  AND granted = true
	) l ON s.indexrelid = l.relation
	WHERE l.relation IS NULL;`

	stats := map[indexIdentifer]indexStat{}

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errs []error
	for rows.Next() {
		var (
			schema, table, index  string
			indexSize, indexScans int64
		)
		err := rows.Scan(&schema, &table, &index, &indexSize, &indexScans)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		stats[indexKey(database, schema, table, index)] = indexStat{
			index:    index,
			table:    table,
			schema:   schema,
			database: database,
			size:     indexSize,
			scans:    indexScans,
		}
	}
	return stats, multierr.Combine(errs...)
}

type functionStat struct {
	function string
	schema   string
	database string
	calls    int64
}

func (c *postgreSQLClient) getFunctionStats(ctx context.Context, database string) (map[functionIdentifer]functionStat, error) {
	query := `WITH overloaded_funcs AS (
 SELECT funcname
   FROM pg_stat_user_functions s
  GROUP BY s.funcname
 HAVING COUNT(*) > 1
)
SELECT s.schemaname,
       CASE WHEN o.funcname IS NULL OR p.proargnames IS NULL THEN p.proname
            ELSE p.proname || '_' || array_to_string(p.proargnames, '_')
        END funcname,
        s.calls
  FROM pg_proc p
  JOIN pg_stat_user_functions s
    ON p.oid = s.funcid
  LEFT JOIN overloaded_funcs o
    ON o.funcname = s.funcname;`

	stats := map[functionIdentifer]functionStat{}

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errs []error
	for rows.Next() {
		var (
			schema, function string
			calls            int64
		)
		err := rows.Scan(&schema, &function, &calls)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		stats[functionKey(database, schema, function)] = functionStat{
			function: function,
			schema:   schema,
			database: database,
			calls:    calls,
		}
	}
	return stats, multierr.Combine(errs...)
}

// vectorSearchStat holds the aggregated pgvector search statistics for a single distance function.
type vectorSearchStat struct {
	// distanceFunction is the pgvector distance function classification (e.g. cosine, l2, hamming).
	distanceFunction string
	// calls is the cumulative number of executions of statements using this distance function.
	calls int64
	// totalExecTime is the cumulative execution time in seconds.
	totalExecTime float64
	// rowsReturned is the cumulative number of rows returned by statements using this distance function.
	rowsReturned int64
}

//go:embed templates/vectorSearchStatsTemplate.tmpl
var vectorSearchStatsQuery string

func (c *postgreSQLClient) getVectorSearchStats(ctx context.Context) ([]vectorSearchStat, error) {
	rows, err := c.client.QueryContext(ctx, vectorSearchStatsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []vectorSearchStat
	var errs error
	for rows.Next() {
		var distanceFunction string
		var calls int64
		var totalExecTimeMs float64
		var rowsReturned int64
		if err := rows.Scan(&distanceFunction, &calls, &totalExecTimeMs, &rowsReturned); err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		stats = append(stats, vectorSearchStat{
			distanceFunction: distanceFunction,
			calls:            calls,
			// pg_stat_statements reports total_exec_time in milliseconds; convert to seconds.
			totalExecTime: totalExecTimeMs / 1000.0,
			rowsReturned:  rowsReturned,
		})
	}
	if err := rows.Err(); err != nil {
		errs = multierr.Append(errs, err)
	}
	return stats, errs
}

// vectorInsertStat holds the aggregated pgvector insert statistics.
type vectorInsertStat struct {
	// rows is the cumulative number of vectors inserted into pgvector tables.
	rows int64
	// totalExecTime is the cumulative execution time in seconds.
	totalExecTime float64
}

//go:embed templates/vectorInsertStatsTemplate.tmpl
var vectorInsertStatsQuery string

func (c *postgreSQLClient) getVectorInsertStats(ctx context.Context) ([]vectorInsertStat, error) {
	rows, err := c.client.QueryContext(ctx, vectorInsertStatsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []vectorInsertStat
	var errs error
	for rows.Next() {
		var insertedRows int64
		var totalExecTimeMs float64
		if err := rows.Scan(&insertedRows, &totalExecTimeMs); err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		stats = append(stats, vectorInsertStat{
			rows: insertedRows,
			// pg_stat_statements reports total_exec_time in milliseconds; convert to seconds.
			totalExecTime: totalExecTimeMs / 1000.0,
		})
	}
	if err := rows.Err(); err != nil {
		errs = multierr.Append(errs, err)
	}
	return stats, errs
}

type bgStat struct {
	checkpointsReq       int64
	checkpointsScheduled int64
	checkpointWriteTime  float64
	checkpointSyncTime   float64
	bgWrites             int64
	bufferBackendWrites  int64
	bufferFsyncWrites    int64
	bufferCheckpoints    int64
	buffersAllocated     int64
	maxWritten           int64
}

func (c *postgreSQLClient) getBGWriterStats(ctx context.Context) (*bgStat, error) {
	version, err := c.getVersion(ctx)
	if err != nil {
		return nil, err
	}

	major, err := parseMajorVersion(version)
	if err != nil {
		return nil, err
	}

	var (
		checkpointsReq, checkpointsScheduled               int64
		checkpointSyncTime, checkpointWriteTime            float64
		bgWrites, bufferCheckpoints, bufferAllocated       int64
		bufferBackendWrites, bufferFsyncWrites, maxWritten int64
	)

	if major < 17 {
		query := `SELECT
		checkpoints_req AS checkpoint_req,
		checkpoints_timed AS checkpoint_scheduled,
		checkpoint_write_time AS checkpoint_duration_write,
		checkpoint_sync_time AS checkpoint_duration_sync,
		buffers_clean AS bg_writes,
		buffers_backend AS backend_writes,
		buffers_backend_fsync AS buffers_written_fsync,
		buffers_checkpoint AS buffers_checkpoints,
		buffers_alloc AS buffers_allocated,
		maxwritten_clean AS maxwritten_count
		FROM pg_stat_bgwriter;`

		row := c.client.QueryRowContext(ctx, query)

		if err := row.Scan(
			&checkpointsReq,
			&checkpointsScheduled,
			&checkpointWriteTime,
			&checkpointSyncTime,
			&bgWrites,
			&bufferBackendWrites,
			&bufferFsyncWrites,
			&bufferCheckpoints,
			&bufferAllocated,
			&maxWritten,
		); err != nil {
			return nil, err
		}
		return &bgStat{
			checkpointsReq:       checkpointsReq,
			checkpointsScheduled: checkpointsScheduled,
			checkpointWriteTime:  checkpointWriteTime,
			checkpointSyncTime:   checkpointSyncTime,
			bgWrites:             bgWrites,
			bufferBackendWrites:  bufferBackendWrites,
			bufferFsyncWrites:    bufferFsyncWrites,
			bufferCheckpoints:    bufferCheckpoints,
			buffersAllocated:     bufferAllocated,
			maxWritten:           maxWritten,
		}, nil
	}
	query := `SELECT
		cp.num_requested AS checkpoint_req,
		cp.num_timed AS checkpoint_scheduled,
		cp.write_time AS checkpoint_duration_write,
		cp.sync_time AS checkpoint_duration_sync,
		cp.buffers_written AS buffers_checkpoints,
		bg.buffers_clean AS bg_writes,
		bg.buffers_alloc AS buffers_allocated,
		bg.maxwritten_clean AS maxwritten_count
		FROM pg_stat_bgwriter bg, pg_stat_checkpointer cp;`

	row := c.client.QueryRowContext(ctx, query)

	if err := row.Scan(
		&checkpointsReq,
		&checkpointsScheduled,
		&checkpointWriteTime,
		&checkpointSyncTime,
		&bufferCheckpoints,
		&bgWrites,
		&bufferAllocated,
		&maxWritten,
	); err != nil {
		return nil, err
	}

	return &bgStat{
		checkpointsReq:       checkpointsReq,
		checkpointsScheduled: checkpointsScheduled,
		checkpointWriteTime:  checkpointWriteTime,
		checkpointSyncTime:   checkpointSyncTime,
		bgWrites:             bgWrites,
		bufferBackendWrites:  -1, // Not found in pg17+ tables
		bufferFsyncWrites:    -1, // Not found in pg17+ tables
		bufferCheckpoints:    bufferCheckpoints,
		buffersAllocated:     bufferAllocated,
		maxWritten:           maxWritten,
	}, nil
}

func (c *postgreSQLClient) getMaxConnections(ctx context.Context) (int64, error) {
	query := `SHOW max_connections;`
	row := c.client.QueryRowContext(ctx, query)
	var maxConns int64
	err := row.Scan(&maxConns)
	return maxConns, err
}

type replicationStats struct {
	clientAddr   string
	pendingBytes int64
	flushLagInt  int64 // Deprecated
	replayLagInt int64 // Deprecated
	writeLagInt  int64 // Deprecated
	flushLag     float64
	replayLag    float64
	writeLag     float64
}

func (c *postgreSQLClient) getDeprecatedReplicationStats(ctx context.Context) ([]replicationStats, error) {
	query := `SELECT
	coalesce(cast(client_addr as varchar), 'unix') AS client_addr,
	coalesce(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), -1) AS replication_bytes_pending,
	extract('epoch' from coalesce(write_lag, '-1 seconds'))::integer,
	extract('epoch' from coalesce(flush_lag, '-1 seconds'))::integer,
	extract('epoch' from coalesce(replay_lag, '-1 seconds'))::integer
	FROM pg_stat_replication;
	`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_stat_replication: %w", err)
	}
	defer rows.Close()
	var rs []replicationStats
	var errors error
	for rows.Next() {
		var client string
		var replicationBytes int64
		var writeLagInt, flushLagInt, replayLagInt int64
		err = rows.Scan(&client, &replicationBytes,
			&writeLagInt, &flushLagInt, &replayLagInt)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		rs = append(rs, replicationStats{
			clientAddr:   client,
			pendingBytes: replicationBytes,
			replayLagInt: replayLagInt,
			writeLagInt:  writeLagInt,
			flushLagInt:  flushLagInt,
		})
	}

	return rs, errors
}

func (c *postgreSQLClient) getReplicationStats(ctx context.Context) ([]replicationStats, error) {
	if !metadata.PostgresqlreceiverPreciselagmetricsFeatureGate.IsEnabled() {
		return c.getDeprecatedReplicationStats(ctx)
	}

	query := `SELECT
	coalesce(cast(client_addr as varchar), 'unix') AS client_addr,
	coalesce(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), -1) AS replication_bytes_pending,
	extract('epoch' from coalesce(write_lag, '-1 seconds'))::decimal AS write_lag_fractional,
	extract('epoch' from coalesce(flush_lag, '-1 seconds'))::decimal AS flush_lag_fractional,
	extract('epoch' from coalesce(replay_lag, '-1 seconds'))::decimal AS replay_lag_fractional
	FROM pg_stat_replication;
	`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_stat_replication: %w", err)
	}
	defer rows.Close()
	var rs []replicationStats
	var errors error
	for rows.Next() {
		var client string
		var replicationBytes int64
		var writeLag, flushLag, replayLag float64
		err = rows.Scan(&client, &replicationBytes, &writeLag, &flushLag, &replayLag)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		rs = append(rs, replicationStats{
			clientAddr:   client,
			pendingBytes: replicationBytes,
			replayLag:    replayLag,
			writeLag:     writeLag,
			flushLag:     flushLag,
		})
	}

	return rs, errors
}

func (c *postgreSQLClient) getLatestWalAgeSeconds(ctx context.Context) (int64, error) {
	query := `SELECT
	coalesce(last_archived_time, CURRENT_TIMESTAMP) AS last_archived_wal,
	CURRENT_TIMESTAMP
	FROM pg_stat_archiver;
	`
	row := c.client.QueryRowContext(ctx, query)
	var lastArchivedWal, currentInstanceTime time.Time
	err := row.Scan(&lastArchivedWal, &currentInstanceTime)
	if err != nil {
		return 0, err
	}

	if lastArchivedWal.Equal(currentInstanceTime) {
		return 0, errNoLastArchive
	}

	age := int64(currentInstanceTime.Sub(lastArchivedWal).Seconds())
	return age, nil
}

func (c *postgreSQLClient) listDatabases(ctx context.Context) ([]string, error) {
	query := `SELECT datname FROM pg_database
	WHERE datistemplate = false;`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}

		databases = append(databases, database)
	}
	return databases, nil
}

func (c *postgreSQLClient) getVersion(ctx context.Context) (string, error) {
	query := "SHOW server_version;"
	row := c.client.QueryRowContext(ctx, query)
	var version string
	err := row.Scan(&version)
	return version, err
}

func parseMajorVersion(ver string) (int, error) {
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected version string: %s", ver)
	}

	return strconv.Atoi(parts[0])
}

// quoteDatabaseList renders databases as SQL string literals for an IN/NOT IN clause.
func quoteDatabaseList(databases []string) string {
	quoted := make([]string, len(databases))
	for i, db := range databases {
		quoted[i] = pq.QuoteLiteral(db)
	}
	return strings.Join(quoted, ",")
}

func filterQueryByDatabases(baseQuery string, databases []string, groupBy bool) string {
	if len(databases) > 0 {
		keyword := " WHERE"
		if strings.Contains(baseQuery, "WHERE") {
			keyword = " AND"
		}
		baseQuery += keyword + " datname IN (" + quoteDatabaseList(databases) + ")"
	}
	if groupBy {
		baseQuery += " GROUP BY datname"
	}

	return baseQuery + ";"
}

func tableKey(database, schema, table string) tableIdentifier {
	return tableIdentifier(fmt.Sprintf("%s|%s|%s", database, schema, table))
}

func indexKey(database, schema, table, index string) indexIdentifer {
	return indexIdentifer(fmt.Sprintf("%s|%s|%s|%s", database, schema, table, index))
}

func functionKey(database, schema, function string) functionIdentifer {
	return functionIdentifer(fmt.Sprintf("%s|%s|%s", database, schema, function))
}

//go:embed templates/querySampleTemplate.tmpl
var querySampleTemplate string

var querySampleTmpl = template.Must(template.New("querySample").Option("missingkey=error").Parse(querySampleTemplate))

func (c *postgreSQLClient) getQuerySamples(ctx context.Context, limit int64, newestQueryTimestamp float64, excludedDatabases []string, logger *zap.Logger) ([]map[string]any, float64, error) {
	buf := bytes.Buffer{}

	if tmplErr := querySampleTmpl.Execute(&buf, map[string]any{
		"limit":                limit,
		"newestQueryTimestamp": newestQueryTimestamp,
		"excludedDatabases":    quoteDatabaseList(excludedDatabases),
	}); tmplErr != nil {
		logger.Error("failed to execute template", zap.Error(tmplErr))
		return []map[string]any{}, newestQueryTimestamp, fmt.Errorf("failed executing template: %w", tmplErr)
	}

	wrappedDb := sqlquery.NewDbClient(sqlquery.DbWrapper{Db: c.client}, buf.String(), logger, sqlquery.TelemetryConfig{})

	rows, err := wrappedDb.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			logger.Error("failed getting log rows", zap.Error(err))
			return []map[string]any{}, newestQueryTimestamp, fmt.Errorf("getQuerySamples failed getting log rows: %w", err)
		}
		// in case the sql returned rows contains null value, we just log a warning and continue
		logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	errs := make([]error, 0)
	finalAttributes := make([]map[string]any, 0)
	propagator := propagation.TraceContext{}
	for _, row := range rows {
		if row[querySampleColumnQuery] == insufficientPrivilegeQuerySampleText {
			logger.Warn("skipping query sample due to insufficient privileges")
			errs = append(errs, errors.New("skipping query sample due to insufficient privileges"))
			continue
		}
		currentAttributes := make(map[string]any)
		var traceCtx context.Context
		querySampleSimpleColumns := []string{
			querySampleColumnClientHostname,
			querySampleColumnQueryStart,
			querySampleColumnWaitEventType,
			querySampleColumnWaitEvent,
			querySampleColumnQueryID,
			querySampleColumnState,
			querySampleColumnApplicationName,
			querySampleColumnBlockingPIDs,
			querySampleColumnBlockingStartTime,
			querySampleColumnBlockingLockMode,
			querySampleColumnBlockingLockType,
			querySampleColumnBlockingLockRelation,
			querySampleColumnBlockingTxnStartTime,
		}

		for _, col := range querySampleSimpleColumns {
			currentAttributes[dbAttributePrefix+col] = row[col]
			if col == querySampleColumnApplicationName && row[col] != "" {
				// Use a background context so we don't accidentally inherit cancellation or span context
				// from the scrape context; the only trace linkage should come from the extracted traceparent.
				ctxFromQuery := propagator.Extract(context.Background(), propagation.MapCarrier{
					traceparentCarrierKey: row[col],
				})

				if trace.SpanContextFromContext(ctxFromQuery).IsValid() {
					traceCtx = ctxFromQuery
				}
			}
		}

		if traceCtx != nil {
			currentAttributes[querySampleTraceContextKey] = traceCtx
		}

		clientPort := int64(0)
		if row[querySampleColumnClientPort] != "" {
			clientPort, err = strconv.ParseInt(row[querySampleColumnClientPort], 10, 64)
			if err != nil {
				logger.Warn("failed to convert client_port to int", zap.Error(err))
				errs = append(errs, err)
			}
		}
		pid := int64(0)
		if row[querySampleColumnPID] != "" {
			pid, err = strconv.ParseInt(row[querySampleColumnPID], 10, 64)
			if err != nil {
				logger.Warn("failed to convert pid to int64", zap.Error(err))
				errs = append(errs, err)
			}
		}
		_queryStartTimestamp := float64(0)
		if row[querySampleColumnQueryStartTimestamp] != "" {
			_queryStartTimestamp, err = strconv.ParseFloat(row[querySampleColumnQueryStartTimestamp], 64)
			if err != nil {
				logger.Warn("failed to convert _query_start_timestamp", zap.Error(err))
				errs = append(errs, err)
			}
		}
		newestQueryTimestamp = math.Max(newestQueryTimestamp, _queryStartTimestamp)

		duration := float64(0)
		if row[querySampleColumnDurationMilliseconds] != "" {
			duration, err = strconv.ParseFloat(row[querySampleColumnDurationMilliseconds], 64)
			if err != nil {
				logger.Warn("failed to convert duration", zap.Error(err))
				errs = append(errs, err)
			}
		}

		blockingWaitDuration := int64(0)
		if row[querySampleColumnBlockingWaitDuration] != "" {
			blockingWaitDuration, err = strconv.ParseInt(row[querySampleColumnBlockingWaitDuration], 10, 64)
			if err != nil {
				logger.Warn("failed to convert blocking_wait_duration to int64", zap.Error(err))
				errs = append(errs, err)
			}
		}

		// TODO: check if the query is truncated.
		obfuscated, err := obfuscateSQL(row[querySampleColumnQuery])
		if err != nil {
			logger.Warn("failed to obfuscate query", zap.String("query", row[querySampleColumnQuery]))
			obfuscated = ""
		}
		currentAttributes[dbAttributePrefix+querySampleColumnPID] = pid
		currentAttributes[string(semconv.NetworkPeerPortKey)] = clientPort
		currentAttributes[string(semconv.NetworkPeerAddressKey)] = row[querySampleColumnClientAddr]
		currentAttributes[string(semconv.DBQueryTextKey)] = obfuscated
		currentAttributes[string(semconv.DBNamespaceKey)] = row[querySampleColumnDatname]
		currentAttributes[string(semconv.UserNameKey)] = row[querySampleColumnUsename]
		currentAttributes[postgresqlTotalExecTimeAttributeName] = duration
		currentAttributes[dbAttributePrefix+querySampleColumnBlockingWaitDuration] = blockingWaitDuration
		finalAttributes = append(finalAttributes, currentAttributes)
	}

	return finalAttributes, newestQueryTimestamp, errors.Join(errs...)
}

func convertMillisecondToSecond(column, value string, logger *zap.Logger) (any, error) {
	result := float64(0)
	var err error
	if value != "" {
		result, err = strconv.ParseFloat(value, 64)
		if err != nil {
			logger.Error("failed to parse float", zap.String("column", column), zap.String("value", value), zap.Error(err))
		}
	}
	return result / 1000.0, err
}

func convertToInt(column, value string, logger *zap.Logger) (any, error) {
	result := 0
	var err error
	if value != "" {
		result, err = strconv.Atoi(value)
		if err != nil {
			logger.Error("failed to parse int", zap.String("column", column), zap.String("value", value), zap.Error(err))
		}
	}
	return int64(result), err
}

//go:embed templates/topQueryTemplate.tmpl
var topQueryTemplate string

var topQueryTmpl = template.Must(template.New("topQuery").Option("missingkey=error").Parse(topQueryTemplate))

// getTopQuery implements client.
func (c *postgreSQLClient) getTopQuery(ctx context.Context, limit int64, excludedDatabases []string, logger *zap.Logger) ([]map[string]any, error) {
	buf := bytes.Buffer{}

	// TODO: Only get query after the oldest query we got from the previous sample query colelction.
	// For instance, if from the last sample query we got queries executed between 8:00 ~ 8:15,
	// in this query, we should only gather query after 8:15
	if err := topQueryTmpl.Execute(&buf, map[string]any{
		"limit":             limit,
		"excludedDatabases": quoteDatabaseList(excludedDatabases),
	}); err != nil {
		logger.Error("failed to execute template", zap.Error(err))
		return []map[string]any{}, fmt.Errorf("failed executing template: %w", err)
	}

	wrappedDb := sqlquery.NewDbClient(sqlquery.DbWrapper{Db: c.client}, buf.String(), logger, sqlquery.TelemetryConfig{})

	rows, err := wrappedDb.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			logger.Error("failed getting log rows", zap.Error(err))
			return []map[string]any{}, fmt.Errorf("getTopQuery failed getting log rows: %w", err)
		}
		// in case the sql returned rows contains null value, we just log a warning and continue
		logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	errs := make([]error, 0)
	finalAttributes := make([]map[string]any, 0)

	for _, row := range rows {
		hasConvention := map[string]string{
			"datname": string(semconv.DBNamespaceKey),
			"query":   string(semconv.DBQueryTextKey),
		}

		needConversion := map[string]func(string, string, *zap.Logger) (any, error){
			callsColumnName:             convertToInt,
			rowsColumnName:              convertToInt,
			sharedBlksDirtiedColumnName: convertToInt,
			sharedBlksHitColumnName:     convertToInt,
			sharedBlksReadColumnName:    convertToInt,
			sharedBlksWrittenColumnName: convertToInt,
			tempBlksReadColumnName:      convertToInt,
			tempBlksWrittenColumnName:   convertToInt,
			totalExecTimeColumnName:     convertMillisecondToSecond,
			totalPlanTimeColumnName:     convertMillisecondToSecond,
		}
		currentAttributes := make(map[string]any)

		// Store raw query before obfuscation (needed for EXPLAIN with $N placeholders)
		if rawQuery, ok := row["query"]; ok {
			currentAttributes[dbAttributePrefix+"raw_query"] = rawQuery
		}

		for col := range row {
			var val any
			var err error
			converter, ok := needConversion[col]
			switch {
			case ok:
				val, err = converter(col, row[col], logger)
				if err != nil {
					logger.Warn("failed to convert column to int", zap.String("column", col), zap.Error(err))
					errs = append(errs, err)
				}
			case col == "query":
				// Obfuscate query for display/logging (converts $1,$2 to ?)
				// Raw query is already stored separately for EXPLAIN
				val, err = obfuscateSQL(row[col])
				if err != nil {
					logger.Error("failed to obfuscate query", zap.String("query", row[col]))
					val = ""
				}
			default:
				val = row[col]
			}
			if hasConvention[col] != "" {
				currentAttributes[hasConvention[col]] = val
			} else {
				currentAttributes[dbAttributePrefix+col] = val
			}
		}
		finalAttributes = append(finalAttributes, currentAttributes)
	}

	return finalAttributes, errors.Join(errs...)
}
