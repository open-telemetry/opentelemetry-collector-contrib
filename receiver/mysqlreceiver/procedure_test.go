// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

const topProcedureEvent = "db.server.top_procedure"

func (c *mockClient) getTopProcedures(uint64) ([]topProcedure, error) {
	return c.topProcedures, c.topProceduresErr
}

func (c *mockClient) getProcedureDefinitions(routines []topProcedure) ([]procedureDefinition, error) {
	c.definitionCalls = append(c.definitionCalls, routines)
	return c.definitions, c.definitionsErr
}

func (c *mockClient) getProcedureStatements(since, _ uint64) ([]procedureStatement, error) {
	c.statementSince = append(c.statementSince, since)
	var out []procedureStatement
	for _, st := range c.procedureStatements {
		if st.timerStart > since {
			out = append(out, st)
		}
	}
	return out, c.procedureStatementsErr
}

func newTopProcedureScraper(t *testing.T, mc *mockClient) *mySQLScraper {
	t.Helper()
	if mc.dbVersionOverride == nil {
		v := mustDBVersion(t, "8.4.0")
		mc.dbVersionOverride = &v
	}
	s := newQueryPlanEventScraper(t, mc)
	s.config.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled = true
	s.config.TopProcedureCollection.CollectionInterval = 0
	// The logs builder captured the config at construction, so rebuild it with the event on.
	s.lb = metadata.NewLogsBuilder(s.config.LogsBuilderConfig, receivertest.NewNopSettings(metadata.Type))
	return s
}

func TestScrapeTopProcedures(t *testing.T) {
	proc := func(count, elapsed int64) topProcedure {
		return topProcedure{
			schemaName: "shop", name: "place_order", objectType: "procedure",
			countStar: count, sumTimerWait: elapsed, sumCPUTime: elapsed / 2, sumErrors: count / 4,
			minTimerWait: 1_000_000_000_000, maxTimerWait: 3_000_000_000_000,
			sumRowsAffected: count * 2, sumRowsExamined: count * 10, sumRowsSent: count * 3,
			sumTmpDiskTables: count / 2, sumSortMergePasses: count / 4,
		}
	}
	fn := topProcedure{schemaName: "shop", name: "place_order", objectType: "function", countStar: 5, sumTimerWait: 5}
	mc := &mockClient{topProcedures: []topProcedure{proc(4, 4_000_000_000_000), fn}}
	s := newTopProcedureScraper(t, mc)

	// Seeding scrape primes the delta cache and reports nothing.
	logs, err := s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)
	assert.Empty(t, recordsByEvent(logs)[topProcedureEvent])

	// A procedure and a function sharing a name are tracked separately: only the procedure moves.
	mc.topProcedures = []topProcedure{proc(8, 10_000_000_000_000), fn}
	logs, err = s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)
	records := recordsByEvent(logs)[topProcedureEvent]
	require.Len(t, records, 1)

	attrs := records[0].Attributes().AsRaw()
	assert.Equal(t, "mysql", attrs["db.system.name"])
	assert.Equal(t, "shop", attrs["db.namespace"])
	assert.Equal(t, "place_order", attrs["mysql.procedure.name"])
	assert.Equal(t, "procedure", attrs["mysql.procedure.type"])
	assert.Equal(t, int64(4), attrs["mysql.procedure.execution_count"])
	assert.InDelta(t, 6.0, attrs["mysql.procedure.total_elapsed_time"], 1e-9)
	assert.InDelta(t, 3.0, attrs["mysql.procedure.cpu_time"], 1e-9)
	assert.Equal(t, int64(1), attrs["mysql.procedure.error_count"])
	assert.InDelta(t, 1.0, attrs["mysql.procedure.min_duration"], 1e-9)
	assert.InDelta(t, 3.0, attrs["mysql.procedure.max_duration"], 1e-9)
	assert.Equal(t, int64(8), attrs["mysql.procedure.rows_affected"])
	assert.Equal(t, int64(40), attrs["mysql.procedure.rows_examined"])
	assert.Equal(t, int64(12), attrs["mysql.procedure.rows_sent"])
	assert.Equal(t, int64(2), attrs["mysql.procedure.tmp_disk_tables"])
	assert.Equal(t, int64(1), attrs["mysql.procedure.sort_merge_passes"])
}

func TestScrapeTopProceduresCountResetAndTopN(t *testing.T) {
	mk := func(name string, count, elapsed int64) topProcedure {
		return topProcedure{schemaName: "s", name: name, objectType: "procedure", countStar: count, sumTimerWait: elapsed}
	}
	mc := &mockClient{topProcedures: []topProcedure{mk("a", 10, 100), mk("b", 10, 100)}}
	s := newTopProcedureScraper(t, mc)
	s.config.TopProcedureCollection.TopProcedureCount = 1

	_, err := s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)

	// "a" reset (counter dropped), so its whole current value is the delta; "b" grew by 50 only.
	mc.topProcedures = []topProcedure{mk("a", 3, 300), mk("b", 15, 150)}
	logs, err := s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)
	records := recordsByEvent(logs)[topProcedureEvent]
	require.Len(t, records, 1, "top_procedure_count must cap the reported routines")
	name, _ := records[0].Attributes().Get("mysql.procedure.name")
	assert.Equal(t, "a", name.Str())
}

func TestScrapeTopProceduresUnsupportedVersion(t *testing.T) {
	for _, v := range []string{"8.0.27", "5.7.38"} {
		t.Run(v, func(t *testing.T) {
			ver := mustDBVersion(t, v)
			mc := &mockClient{dbVersionOverride: &ver, topProceduresErr: errors.New("must not be queried")}
			s := newTopProcedureScraper(t, mc)
			logs, err := s.scrapeTopProcedureFunc(t.Context())
			require.NoError(t, err)
			assert.Empty(t, recordsByEvent(logs)[topProcedureEvent])
		})
	}
}

func TestScrapeTopProceduresQueryError(t *testing.T) {
	s := newTopProcedureScraper(t, &mockClient{topProceduresErr: errors.New("denied")})
	_, err := s.scrapeTopProcedureFunc(t.Context())
	require.Error(t, err)
}

func TestConfigValidate_TopProcedureCollection(t *testing.T) {
	valid := func() *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled = true
		return cfg
	}
	require.NoError(t, valid().Validate())

	cfg := valid()
	cfg.TopProcedureCollection.TopProcedureCount = 0
	require.ErrorContains(t, cfg.Validate(), "must be greater than 0")

	cfg = valid()
	cfg.TopProcedureCollection.TopProcedureCount = cfg.TopProcedureCollection.MaxProcedureSampleCount + 1
	require.ErrorContains(t, cfg.Validate(), "less than or equal to")

	cfg = valid()
	cfg.TopProcedureCollection.MaxProcedureSampleCount = 10001
	require.ErrorContains(t, cfg.Validate(), "between 1 and 10000")

	// Zero counts are tolerated while the event is off.
	cfg = valid()
	cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled = false
	cfg.TopProcedureCollection.TopProcedureCount = 0
	require.NoError(t, cfg.Validate())
}

func TestSupportsProcedureStats(t *testing.T) {
	assert.True(t, mustDBVersion(t, "8.4.0").supportsProcedureStats())
	assert.True(t, mustDBVersion(t, "8.0.28").supportsProcedureStats())
	assert.False(t, mustDBVersion(t, "8.0.27").supportsProcedureStats())
	assert.False(t, dbVersion{}.supportsProcedureStats())
}

func TestScrapeProcedureDefinitions(t *testing.T) {
	proc := func(name string, count int64) topProcedure {
		return topProcedure{schemaName: "shop", name: name, objectType: "procedure", countStar: count, sumTimerWait: count}
	}
	mc := &mockClient{
		topProcedures: []topProcedure{proc("a", 1), proc("idle", 1), proc("hidden", 1)},
		definitions: []procedureDefinition{
			{schemaName: "shop", name: "a", objectType: "procedure", definition: "BEGIN SELECT * FROM t WHERE pw = 'hunter2'; END"},
			{schemaName: "shop", name: "hidden", objectType: "procedure", definition: ""},
		},
	}
	s := newTopProcedureScraper(t, mc)
	s.config.LogsBuilderConfig.Events.DbServerProcedureDefinition.Enabled = true
	s.config.TopProcedureCollection.DefinitionMaxLength = 20
	s.lb = metadata.NewLogsBuilder(s.config.LogsBuilderConfig, receivertest.NewNopSettings(metadata.Type))

	_, err := s.scrapeTopProcedureFunc(t.Context()) // seeding scrape
	require.NoError(t, err)
	require.Empty(t, mc.definitionCalls, "no routine was reported, so no body is fetched")

	mc.topProcedures = []topProcedure{proc("a", 2), proc("idle", 1), proc("hidden", 2)}
	logs, err := s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)
	require.Len(t, mc.definitionCalls, 1)
	require.Len(t, mc.definitionCalls[0], 2, "only routines reported on top_procedure are looked up")

	records := recordsByEvent(logs)["db.server.procedure_definition"]
	require.Len(t, records, 1, "a routine with a hidden body emits no definition")
	attrs := records[0].Attributes().AsRaw()
	assert.Equal(t, "a", attrs["mysql.procedure.name"])
	assert.Equal(t, "shop", attrs["db.namespace"])
	assert.Equal(t, "procedure", attrs["mysql.procedure.type"])
	body, _ := attrs["mysql.procedure.definition"].(string)
	assert.NotContains(t, body, "hunter2")
	assert.LessOrEqual(t, len([]rune(body)), 20)
	assert.True(t, s.warnedHiddenDefinitions)
}

func TestConfigValidate_ProcedureDefinition(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.LogsBuilderConfig.Events.DbServerProcedureDefinition.Enabled = true
	require.ErrorContains(t, cfg.Validate(), ErrProcedureDefinitionWithoutSource)

	cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled = true
	require.NoError(t, cfg.Validate())

	cfg.TopProcedureCollection.DefinitionMaxLength = 0
	require.ErrorContains(t, cfg.Validate(), "definition_max_length")
}

func TestScrapeProcedureStatements(t *testing.T) {
	proc := func(name string, count int64) topProcedure {
		return topProcedure{schemaName: "shop", name: name, objectType: "procedure", countStar: count, sumTimerWait: count}
	}
	stmt := func(name, text string, start uint64, wait int64) procedureStatement {
		return procedureStatement{
			schemaName: "shop", name: name, objectType: "procedure", sqlText: text, timerStart: start,
			timerWait: wait, cpuTime: wait / 2, rowsExamined: 10, rowsSent: 1, rowsAffected: 2,
		}
	}
	mc := &mockClient{
		topProcedures: []topProcedure{proc("a", 1), proc("idle", 1)},
		procedureStatements: []procedureStatement{
			stmt("a", "SELECT * FROM t WHERE pw = 'hunter2'", 10, 1_000_000_000_000),
			stmt("a", "SELECT * FROM t WHERE pw = 'other'", 20, 3_000_000_000_000),
			stmt("a", "UPDATE t SET v = 1", 30, 2_000_000_000_000),
			stmt("idle", "SELECT 1", 40, 1),
		},
	}
	s := newTopProcedureScraper(t, mc)
	s.config.LogsBuilderConfig.Events.DbServerProcedureStatement.Enabled = true
	s.lb = metadata.NewLogsBuilder(s.config.LogsBuilderConfig, receivertest.NewNopSettings(metadata.Type))

	_, err := s.scrapeTopProcedureFunc(t.Context()) // seeding scrape reports no routine
	require.NoError(t, err)
	require.Empty(t, mc.statementSince, "no routine was reported, so history is not read")

	mc.topProcedures = []topProcedure{proc("a", 2), proc("idle", 1)}
	logs, err := s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)

	records := recordsByEvent(logs)["db.server.procedure_statement"]
	require.Len(t, records, 2, "statements with the same obfuscated text are grouped, idle routines are skipped")
	attrs := records[0].Attributes().AsRaw()
	assert.Equal(t, "a", attrs["mysql.procedure.name"])
	assert.Equal(t, "shop", attrs["db.namespace"])
	assert.Equal(t, int64(2), attrs["mysql.procedure.execution_count"])
	assert.InDelta(t, 4.0, attrs["mysql.procedure.total_elapsed_time"], 1e-9)
	assert.InDelta(t, 2.0, attrs["mysql.procedure.cpu_time"], 1e-9)
	assert.Equal(t, int64(20), attrs["mysql.procedure.rows_examined"])
	assert.Equal(t, int64(2), attrs["mysql.procedure.rows_sent"])
	assert.Equal(t, int64(4), attrs["mysql.procedure.rows_affected"])
	text, _ := attrs["mysql.procedure.statement"].(string)
	assert.NotContains(t, text, "hunter2")
	assert.NotContains(t, text, "other")

	// Rows already emitted are not counted again.
	mc.topProcedures = []topProcedure{proc("a", 3), proc("idle", 1)}
	logs, err = s.scrapeTopProcedureFunc(t.Context())
	require.NoError(t, err)
	assert.Empty(t, recordsByEvent(logs)["db.server.procedure_statement"])
	assert.Equal(t, uint64(40), s.lastProcedureStatementStart)
}

func TestConfigValidate_ProcedureStatement(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.LogsBuilderConfig.Events.DbServerProcedureStatement.Enabled = true
	require.ErrorContains(t, cfg.Validate(), ErrProcedureStatementWithoutSource)

	cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled = true
	require.NoError(t, cfg.Validate())

	cfg.TopProcedureCollection.DefinitionMaxLength = 0
	require.ErrorContains(t, cfg.Validate(), "definition_max_length")
}
