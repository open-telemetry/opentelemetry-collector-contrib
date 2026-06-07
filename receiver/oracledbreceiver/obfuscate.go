// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"strings"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	// collectCommentsConfig extracts comments into metadata so they can be located in the original SQL.
	collectCommentsConfig = obfuscate.SQLConfig{
		DBMS:            "oracle",
		ObfuscationMode: "obfuscate_and_normalize",
		CollectComments: true,
		KeepSQLAlias:    true,
		KeepBoolean:     true,
		KeepNull:        true,
	}

	// obfuscateSQLConfig replaces literals with ? while preserving the query structure.
	obfuscateSQLConfig = obfuscate.SQLConfig{
		DBMS:            "oracle",
		ObfuscationMode: "obfuscate_only",
		KeepSQLAlias:    true,
		KeepBoolean:     true,
		KeepNull:        true,
	}
)

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		SQL: obfuscateSQLConfig,
	}))
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	// Comments cannot be obfuscated in place, so collect them first and replace
	// each occurrence with a ? placeholder before obfuscating the remaining literals.
	collectResult, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sql, &collectCommentsConfig, "")
	if err != nil {
		return "", err
	}

	sqlWithAnonymizedComments := sql
	for _, comment := range collectResult.Metadata.Comments {
		sqlWithAnonymizedComments = strings.Replace(sqlWithAnonymizedComments, comment, "?", 1)
	}

	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sqlWithAnonymizedComments, &obfuscateSQLConfig, "")
	if err != nil {
		return "", err
	}

	return obfuscatedQuery.Query, nil
}
