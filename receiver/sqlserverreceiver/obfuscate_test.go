// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// ConstValue with content that the obfuscator can't parse as SQL
	// should be replaced with "?" rather than aborting the whole plan
	plan := `<ShowPlanXML><StmtSimple StatementText="SELECT 1" ConstValue="` +
		strings.Repeat("((((", 500) + `"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	require.NoError(t, err)
	// The plan should still be returned (possibly with "?" for the failed attr)
	// and must NOT contain the raw deeply-nested parentheses
	assert.NotContains(t, result, strings.Repeat("((((", 500))
}

func TestObfuscateXMLPlan_NoStdoutOnFailure(t *testing.T) {
	o := newObfuscator()
	plan := `<ShowPlanXML><StmtSimple ConstValue="` +
		strings.Repeat("((((", 500) + `"/></ShowPlanXML>`
	result, err := o.obfuscateXMLPlan(plan)
	// Should not error out
	require.NoError(t, err)
	// Should not contain raw data in the result
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
