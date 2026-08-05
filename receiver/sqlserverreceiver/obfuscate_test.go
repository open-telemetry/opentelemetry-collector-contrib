// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestObfuscateSQL(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "expectedSQL.sql"))
	assert.NoError(t, err)
	expectedSQL := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "inputSQL.sql"))
	assert.NoError(t, err)

	result, err := newObfuscator(zap.NewNop()).obfuscateSQLString(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedSQL, result)
}

func TestObfuscateInvalidSQL(t *testing.T) {
	obf := newObfuscator(zap.NewNop())
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

	result, err := newObfuscator(zap.NewNop()).obfuscateXMLPlan(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedQueryPlan, result)
}

func TestInvalidQueryPlans(t *testing.T) {
	obf := newObfuscator(zap.NewNop())

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

	// obfuscate failure: the failing attribute is redacted and the rest of the plan is preserved
	plan = `<ShowPlanXML StatementText="[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]/(10000)*(3600)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(10000)/(100)*(60)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(100)"></ShowPlanXML>`
	result, err = obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	assert.Equal(t, `<ShowPlanXML StatementText="?"></ShowPlanXML>`, result)
}

func TestValidQueryPlans(t *testing.T) {
	obf := newObfuscator(zap.NewNop())

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
