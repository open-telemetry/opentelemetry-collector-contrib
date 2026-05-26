// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var collectCommentsConfig = obfuscate.SQLConfig{
	DBMS:            "mssql",
	ObfuscationMode: "obfuscate_and_normalize",
	CollectComments: true,
	KeepSQLAlias:    true,
	KeepBoolean:     true,
	KeepNull:        true,
}

var fullQueryTextObfuscateConfig = obfuscate.SQLConfig{
	DBMS:            "mssql",
	ObfuscationMode: "obfuscate_only",
	KeepSQLAlias:    true,
	KeepBoolean:     true,
	KeepNull:        true,
}

var xmlPlanObfuscationAttrs = []string{
	"StatementText",
	"ConstValue",
	"ScalarString",
	"ParameterCompiledValue",
}

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		SQL: obfuscate.SQLConfig{
			DBMS: "mssql",
		},
	}))
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLString(sql)
	if err != nil {
		return "", err
	}
	return obfuscatedQuery.Query, nil
}

// obfuscateFullSQLString obfuscates a full SQL batch text using a two-step approach:
// Step 1: collect comments and replace them with ? placeholders
// Step 2: obfuscate literals using obfuscate_only mode
func (o *obfuscator) obfuscateFullSQLString(sql string) (string, error) {
	collectResult, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sql, &collectCommentsConfig, "")
	if err != nil {
		return "", err
	}

	sqlWithAnonymizedComments := sql
	for _, comment := range collectResult.Metadata.Comments {
		sqlWithAnonymizedComments = strings.Replace(sqlWithAnonymizedComments, comment, "?", 1)
	}

	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sqlWithAnonymizedComments, &fullQueryTextObfuscateConfig, "")
	if err != nil {
		return "", err
	}

	return obfuscatedQuery.Query, nil
}

// obfuscateXMLPlan obfuscates SQL text & parameters from the provided SQL Server XML Plan
func (o *obfuscator) obfuscateXMLPlan(rawPlan string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(rawPlan))
	var buffer bytes.Buffer
	encoder := xml.NewEncoder(&buffer)

	for {
		token, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			for i := range elem.Attr {
				for _, attrName := range xmlPlanObfuscationAttrs {
					if elem.Attr[i].Name.Local == attrName {
						if elem.Attr[i].Value == "" {
							continue
						}
						val, err := o.obfuscateSQLString(elem.Attr[i].Value)
						if err != nil {
							fmt.Println("Unable to obfuscate SQL statement in query plan, skipping: " + elem.Attr[i].Value)
							return "", nil
						}
						elem.Attr[i].Value = val
					}
				}
			}
			err := encoder.EncodeToken(elem)
			if err != nil {
				return "", err
			}
		case xml.CharData:
			elem = bytes.TrimSpace(elem)
			err := encoder.EncodeToken(elem)
			if err != nil {
				return "", err
			}
		case xml.EndElement:
			err := encoder.EncodeToken(elem)
			if err != nil {
				return "", err
			}
		default:
			err := encoder.EncodeToken(token)
			if err != nil {
				return "", err
			}
		}
	}

	err := encoder.Flush()
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
