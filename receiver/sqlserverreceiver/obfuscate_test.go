// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	obf := newObfuscator(zap.NewNop())

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

	result, err := newObfuscator(zap.NewNop()).obfuscateXMLPlan(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedQueryPlan, result)
	assertNamespacesPreserved(t, string(input), result)
}

// xmlShape is the namespace-aware shape of one element: its qualified name and
// the qualified names of its non-declaration attributes, in document order.
type xmlShape struct {
	space, local string
	attrs        []xml.Name
}

// decodeShape walks doc and returns the shape of every element in document
// order, how many times the root declares the default namespace, and the names
// of any descendants that re-declare the root's default namespace. Go's decoder
// accepts duplicate xmlns attributes, so the well-formedness claim is carried by
// the declaration counts, not by the decode succeeding.
func decodeShape(t *testing.T, doc string) (shape []xmlShape, rootDecls int, redeclared []string) {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(doc))
	depth := 0
	rootNS := ""
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		switch elem := token.(type) {
		case xml.StartElement:
			var attrs []xml.Name
			for _, a := range elem.Attr {
				isDefaultDecl := a.Name.Space == "" && a.Name.Local == "xmlns"
				if isDefaultDecl || a.Name.Space == "xmlns" {
					if isDefaultDecl && depth == 0 {
						rootDecls++
						rootNS = a.Value
					}
					if isDefaultDecl && depth > 0 && a.Value == rootNS {
						redeclared = append(redeclared, elem.Name.Local)
					}
					continue
				}
				attrs = append(attrs, a.Name)
			}
			shape = append(shape, xmlShape{space: elem.Name.Space, local: elem.Name.Local, attrs: attrs})
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return shape, rootDecls, redeclared
}

// assertNamespacesPreserved checks that obfuscation changed attribute values
// only: every element and attribute resolves to the same qualified name as in
// the input, the root declares the default namespace exactly once, and no
// descendant re-declares it.
func assertNamespacesPreserved(t *testing.T, input, result string) {
	t.Helper()
	inShape, _, _ := decodeShape(t, input)
	outShape, rootDecls, redeclared := decodeShape(t, result)
	assert.Equal(t, inShape, outShape, "obfuscation must not change element or attribute qualified names")
	assert.Equal(t, 1, rootDecls, "root must declare the default namespace exactly once")
	assert.Empty(t, redeclared, "no descendant may re-declare the root's default namespace")
}

func TestObfuscateQueryPlanPreservesNamespaces(t *testing.T) {
	const showplan = "http://schemas.microsoft.com/sqlserver/2004/07/showplan"
	tests := map[string]string{
		"prefixed declarations on the root": `<ShowPlanXML xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" Version="1.564" Build="16.0.4150.1" xmlns="` + showplan + `"><BatchSequence><Batch><Statements><StmtSimple StatementText="SELECT * from table where value = 120" StatementId="1"/></Statements></Batch></BatchSequence></ShowPlanXML>`,
		"nested namespace scope and reset":  `<ShowPlanXML xmlns="` + showplan + `" Version="1.564"><BatchSequence><Batch><Statements><StmtSimple StatementText="SELECT 1"><InternalInfo xmlns="` + showplan + `/internalinfo"><Field FieldName="wszWarning" FieldValue="none"/></InternalInfo><NoNamespaceChild xmlns=""><Inner Attr="v"/></NoNamespaceChild></StmtSimple></Statements></Batch></BatchSequence></ShowPlanXML>`,
		"xsi:type on a descendant":          `<ShowPlanXML xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="` + showplan + `" Version="1.564"><BatchSequence><Batch><Statements><StmtSimple StatementText="SELECT 1"><QueryPlan><RelOp xsi:type="Scan" NodeId="0"/></QueryPlan></StmtSimple></Statements></Batch></BatchSequence></ShowPlanXML>`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := newObfuscator(zap.NewNop()).obfuscateXMLPlan(input)
			require.NoError(t, err)
			assertNamespacesPreserved(t, input, result)
		})
	}
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

func TestSanitizeSQL(t *testing.T) {
	obf := newObfuscator(zap.NewNop())

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
	obf := newObfuscator(zap.NewNop())

	plan := "<ShowPlanXML StatementText=\"SELECT \u200b* FROM table\"></ShowPlanXML>"
	result, err := obf.obfuscateXMLPlan(plan)
	assert.NoError(t, err)
	assert.Equal(t, `<ShowPlanXML StatementText="SELECT * FROM table"></ShowPlanXML>`, result)
}

func TestObfuscateSQLServerBackslashLiteral(t *testing.T) {
	obf := newObfuscator(zap.NewNop())

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
