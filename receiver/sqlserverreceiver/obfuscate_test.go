// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

func TestObfuscateSQL(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "expectedSQL.sql"))
	assert.NoError(t, err)
	expectedSQL := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "inputSQL.sql"))
	assert.NoError(t, err)

	result, err := newObfuscator(zap.NewNop(), false).obfuscateSQLString(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedSQL, result)
}

func TestObfuscateInvalidSQL(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	// The go-sqllexer engine (ObfuscateAndNormalize) is tolerant of malformed
	// SQL: instead of failing, it obfuscates what it can. An unclosed bracket
	// identifier no longer produces an error (it did with the legacy tokenizer),
	// so the statement is passed through rather than dropped.
	sql := "SELECT cpu_time AS [CPU Usage (time)"
	result, err := obf.obfuscateSQLString(sql)
	assert.NoError(t, err)
	assert.Equal(t, "SELECT cpu_time AS [CPU Usage (time)", result)

	// Aliases are stripped during normalization.
	sql = "SELECT cpu_time AS [CPU Usage Time]"
	expected := "SELECT cpu_time"
	result, err = obf.obfuscateSQLString(sql)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestObfuscateCommentOnlyStatement(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	// Comment-only statements (e.g. Blue Prism banners captured in
	// sys.dm_exec_sql_text) have no obfuscatable content. The legacy tokenizer
	// returned a "result is empty" error for these, which the scraper logged at
	// error level every scrape interval. The ObfuscateAndNormalize engine
	// returns an empty string with no error, which is the correct benign outcome.
	for _, sql := range []string{
		"--*INSERT-----------",
		"--*SELECT-----------",
		"--*UPDATE-----------",
		"/* banner only */",
		"-- a line comment",
	} {
		result, err := obf.obfuscateSQLString(sql)
		assert.NoError(t, err, "comment-only statement should not error: %q", sql)
		assert.Empty(t, result, "comment-only statement should obfuscate to empty: %q", sql)
	}
}

func TestObfuscateQueryPlan(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "expectedQueryPlan.xml"))
	assert.NoError(t, err)
	expectedQueryPlan := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "inputQueryPlan.xml"))
	assert.NoError(t, err)

	result, err := newObfuscator(zap.NewNop(), false).obfuscateXMLPlan(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedQueryPlan, result)
}

func TestObfuscateQueryPlanRepeatedSensitiveValue(t *testing.T) {
	plan := `<ShowPlanXML><Const ConstValue="42"/><Const ConstValue="42"/></ShowPlanXML>`

	result, err := newObfuscator(zap.NewNop(), false).obfuscateXMLPlan(plan)

	assert.NoError(t, err)
	assert.NotContains(t, result, "42")
	assert.Equal(t, 2, strings.Count(result, `ConstValue="?"`))
}

func TestObfuscateQueryPlanCacheUsesExactXMLAndKeepsOnlyRedactedOutput(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), true)
	plan := `<ShowPlanXML Id="one" StatementText="SELECT 42"></ShowPlanXML>`

	first, err := obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	second, err := obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, obf.xmlPlanCache.Len())

	changed, err := obf.obfuscateXMLPlan(strings.Replace(plan, `Id="one"`, `Id="two"`, 1))
	assert.NoError(t, err)
	assert.NotEqual(t, first, changed)
	assert.Equal(t, 2, obf.xmlPlanCache.Len())
	for _, cached := range obf.xmlPlanCache.Values() {
		assert.NotContains(t, cached, "42")
	}
}

func TestObfuscateQueryPlanCacheIsBoundedAndDoesNotCacheErrors(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), true)
	_, err := obf.obfuscateXMLPlan(`<ShowPlanXML>`)
	assert.Error(t, err)
	assert.Zero(t, obf.xmlPlanCache.Len())

	for i := range 301 {
		plan := fmt.Sprintf(`<ShowPlanXML Id="%d" StatementText="SELECT 42"></ShowPlanXML>`, i)
		_, err = obf.obfuscateXMLPlan(plan)
		assert.NoError(t, err)
	}
	assert.Equal(t, 300, obf.xmlPlanCache.Len())
	first := sha256.Sum256([]byte(`<ShowPlanXML Id="0" StatementText="SELECT 42"></ShowPlanXML>`))
	second := sha256.Sum256([]byte(`<ShowPlanXML Id="1" StatementText="SELECT 42"></ShowPlanXML>`))
	assert.False(t, obf.xmlPlanCache.Contains(first))
	assert.True(t, obf.xmlPlanCache.Contains(second))

	oversized := `<ShowPlanXML Data="` + strings.Repeat("x", maxCachedXMLPlanBytes) + `"></ShowPlanXML>`
	_, err = obf.obfuscateXMLPlan(oversized)
	assert.NoError(t, err)
	assert.Equal(t, 300, obf.xmlPlanCache.Len())
}

func TestObfuscateQueryPlanCacheRefreshesRecentlyUsedPlan(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), true)
	for i := range xmlPlanCacheEntries {
		plan := fmt.Sprintf(`<ShowPlanXML Id="%d" StatementText="SELECT 42"></ShowPlanXML>`, i)
		_, err := obf.obfuscateXMLPlan(plan)
		require.NoError(t, err)
	}
	firstPlan := `<ShowPlanXML Id="0" StatementText="SELECT 42"></ShowPlanXML>`
	_, err := obf.obfuscateXMLPlan(firstPlan)
	require.NoError(t, err)
	_, err = obf.obfuscateXMLPlan(`<ShowPlanXML Id="new" StatementText="SELECT 42"></ShowPlanXML>`)
	require.NoError(t, err)

	first := sha256.Sum256([]byte(firstPlan))
	second := sha256.Sum256([]byte(`<ShowPlanXML Id="1" StatementText="SELECT 42"></ShowPlanXML>`))
	assert.True(t, obf.xmlPlanCache.Contains(first))
	assert.False(t, obf.xmlPlanCache.Contains(second))
}

func TestObfuscateQueryPlanCacheReportsOutcomes(t *testing.T) {
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, telemetry.Shutdown(context.WithoutCancel(t.Context()))) })
	obf := newObfuscator(zap.NewNop(), true)
	obf.initCacheMetrics(telemetry.NewTelemetrySettings(), component.MustNewIDWithName("sql_server", "cache-test"))
	plan := `<ShowPlanXML StatementText="SELECT 42"></ShowPlanXML>`
	for range 2 {
		_, err := obf.obfuscateXMLPlan(plan)
		require.NoError(t, err)
	}
	_, err := obf.obfuscateXMLPlan(`<ShowPlanXML Data="` + strings.Repeat("x", maxCachedXMLPlanBytes) + `"/>`)
	require.NoError(t, err)

	got, err := telemetry.GetMetric("otelcol_sqlserver_xml_plan_cache_requests")
	require.NoError(t, err)
	points := got.Data.(metricdata.Sum[int64]).DataPoints
	require.Len(t, points, 3)
	counts := make(map[string]int64, len(points))
	for _, point := range points {
		outcome, ok := point.Attributes.Value(attribute.Key("outcome"))
		require.True(t, ok)
		receiver, ok := point.Attributes.Value(attribute.Key("receiver"))
		require.True(t, ok)
		assert.Equal(t, "sql_server/cache-test", receiver.AsString())
		counts[outcome.AsString()] = point.Value
	}
	assert.Equal(t, map[string]int64{"hit": 1, "miss": 1, "bypass": 1}, counts)
}

func TestObfuscateQueryPlanCacheDisabled(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)
	plan := `<ShowPlanXML StatementText="SELECT 42"/>`
	first, err := obf.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	second, err := obf.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Nil(t, obf.xmlPlanCache)
}

func TestInvalidQueryPlans(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	plan := `<ShowPlanXml</ShowPlanXML>`
	result, err := obf.obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	plan = `<ShowPlanXML></ShowPlanXML`
	result, err = obf.obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	plan = `<ShowPlanXML></ShowPlan>`
	result, err = obf.obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	// A StatementText that the legacy tokenizer could not obfuscate (and would be
	// redacted to "?" by the #50070 fallback) is now obfuscated successfully by
	// the go-sqllexer engine, so the plan retains the useful normalized statement
	// with its literals redacted rather than losing the attribute entirely.
	plan = `<ShowPlanXML StatementText="[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]/(10000)*(3600)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(10000)/(100)*(60)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(100)"></ShowPlanXML>`
	result, err = obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	assert.Equal(t, `<ShowPlanXML StatementText="msdb.dbo.sysjobhistory.run_duration / ( ? ) * ( ? ) + msdb.dbo.sysjobhistory.run_duration % ( ? ) / ( ? ) * ( ? ) + msdb.dbo.sysjobhistory.run_duration % ( ? )"></ShowPlanXML>`, result)
}

func TestValidQueryPlans(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	plan := `<ShowPlanXML value="abc"></ShowPlanXML>`
	_, err := obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)

	plan = `<ShowPlanXML StatementText=""></ShowPlanXML>`
	_, err = obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)

	plan = `<ShowPlanXML StatementText="SELECT * FROM table"><!-- comment --></ShowPlanXML>`
	_, err = obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
}

func TestSanitizeSQL(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "no zero width characters",
			sql:      "SELECT * FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "zero width space",
			sql:      "SELECT \u200b* FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "zero width non-joiner",
			sql:      "SELECT \u200c* FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "zero width joiner",
			sql:      "SELECT \u200d* FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "byte order mark",
			sql:      "\ufeffSELECT * FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "word joiner",
			sql:      "SELECT \u2060* FROM table",
			expected: "SELECT * FROM table",
		},
		{
			name:     "right to left override",
			sql:      "SELECT \u202e* FROM table",
			expected: "SELECT * FROM table",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeSQL(tt.sql))
		})
	}

	// A statement containing a zero-width space (as seen in Blue Prism work-queue
	// statements from sys.dm_exec_sql_text) should obfuscate successfully after
	// sanitization instead of failing.
	statement := "SELECT \u200b[WQ_Definition] FROM [BluePrism].[WorkQueue]"
	result, err := obf.obfuscateSQLString(statement)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestObfuscateQueryPlanWithZeroWidthSpace(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	plan := "<ShowPlanXML StatementText=\"SELECT \u200b* FROM table\"></ShowPlanXML>"
	result, err := obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	assert.Equal(t, `<ShowPlanXML StatementText="SELECT * FROM table"></ShowPlanXML>`, result)
}

func TestObfuscateSQLServerBackslashLiteral(t *testing.T) {
	obf := newObfuscator(zap.NewNop(), false)

	result, err := obf.obfuscateSQLString(
		`SELECT REPLACE(@@SERVERNAME, '\', ':'), HOST_NAME(), 42`,
	)

	require.NoError(t, err)
	assert.Equal(
		t,
		"SELECT REPLACE ( @@SERVERNAME, ?, ? ), HOST_NAME ( ), ?",
		result,
	)
}
