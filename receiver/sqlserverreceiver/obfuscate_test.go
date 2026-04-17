// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObfuscateSQL(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "expectedSQL.sql"))
	assert.NoError(t, err)
	expectedSQL := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "inputSQL.sql"))
	assert.NoError(t, err)

	result, err := newObfuscator().obfuscateSQLString(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedSQL, result)
}

func TestObfuscateInvalidSQL(t *testing.T) {
	obf := newObfuscator()
	sql := "SELECT cpu_time AS [CPU Usage (time)"
	result, err := obf.obfuscateSQLString(sql)

	assert.Error(t, err)
	assert.Empty(t, result)

	sql = "SELECT cpu_time AS [CPU Usage Time]"
	expected := "SELECT cpu_time"
	result, err = obf.obfuscateSQLString(sql)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestObfuscateQueryPlan(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "expectedQueryPlan.xml"))
	assert.NoError(t, err)
	expectedQueryPlan := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "inputQueryPlan.xml"))
	assert.NoError(t, err)

	result, err := newObfuscator().obfuscateXMLPlan(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedQueryPlan, result)
}

func TestInvalidQueryPlans(t *testing.T) {
	obf := newObfuscator()

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

	// When obfuscation of a StatementText attribute fails, the attribute is
	// replaced with a safe "?" placeholder and the plan is still returned
	// (non-empty). This ensures no raw SQL leaks out of the function.
	plan = `<ShowPlanXML StatementText="[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]/(10000)*(3600)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(10000)/(100)*(60)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(100)"></ShowPlanXML>`
	result, err = obf.obfuscateXMLPlan(plan)
	assert.NotEmpty(t, result, "plan should still be returned with placeholder")
	assert.Contains(t, result, `StatementText="?"`, "failing attr should be replaced with ?")
	assert.NoError(t, err)
}

func TestValidQueryPlans(t *testing.T) {
	obf := newObfuscator()

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

func TestObfuscateSQLString(t *testing.T) {
	o := newObfuscator()
	result, err := o.obfuscateSQLString("SELECT * FROM users WHERE id = 42")
	require.NoError(t, err)
	assert.NotContains(t, result, "42")
}

func TestObfuscateXMLPlan_Success(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple StatementText="SELECT * FROM users WHERE id = 42"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.NotContains(t, result, "42")
	assert.NotEmpty(t, result)
}

func TestObfuscateXMLPlan_FailedAttrGetsPlaceholder(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple StatementText="SELECT 1" ConstValue="` +
		strings.Repeat("((((", 500) + `"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.NotContains(t, result, strings.Repeat("((((", 500))
}

func TestObfuscateXMLPlan_NoStdoutOnFailure(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple ConstValue="` +
		strings.Repeat("((((", 500) + `"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.NotContains(t, result, strings.Repeat("((((", 500))
}

func TestObfuscateXMLPlan_EmptyAttrSkipped(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple StatementText="" ConstValue=""/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestObfuscateXMLPlan_EmptyInput(t *testing.T) {
	o := newObfuscator()
	result, err := o.obfuscateXMLPlan("")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestObfuscateXMLPlan_InvalidXML(t *testing.T) {
	o := newObfuscator()
	_, err := o.obfuscateXMLPlan("<unclosed")
	require.Error(t, err)
}

func TestObfuscateXMLPlan_PreservesNonTargetAttrs(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple QueryHash="ABC123" StatementText="SELECT * FROM t WHERE x = 1"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	assert.Contains(t, result, "ABC123")
	assert.NotContains(t, result, "x = 1")
}
