// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"strings"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	// Config for collecting comments - used in step 1
	collectCommentsConfig = obfuscate.SQLConfig{
		DBMS:            "oracle",
		ObfuscationMode: "obfuscate_and_normalize",
		CollectComments: true, // Extract comments as metadata
		KeepSQLAlias:    true,
		KeepBoolean:     true,
		KeepNull:        true,
	}

	// Config for obfuscating literals - used in step 2
	obfuscateSQLConfig = obfuscate.SQLConfig{
		DBMS:            "oracle",
		ObfuscationMode: "obfuscate_only", // Preserve structure, replace literals with ?
		KeepSQLAlias:    true,             // Preserve AS aliases
		KeepBoolean:     true,             // Preserve TRUE/FALSE literals
		KeepNull:        true,             // Preserve NULL literals
	}
)

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		SQL: obfuscateSQLConfig,
	}))
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	// Two-step approach to anonymize both comments and literals:
	//
	// Step 1: Collect comments using obfuscate_and_normalize mode
	// This extracts comments into metadata while removing them from the query
	collectResult, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sql, &collectCommentsConfig, "")
	if err != nil {
		return "", err
	}

	// Step 2: Replace comments with ? in the original SQL
	sqlWithAnonymizedComments := sql
	for _, comment := range collectResult.Metadata.Comments {
		// Replace each comment with a single ? placeholder
		sqlWithAnonymizedComments = strings.Replace(sqlWithAnonymizedComments, comment, "?", 1)
	}

	// Step 3: Obfuscate literals in the modified SQL using obfuscate_only mode
	// This preserves the structure and ? placeholders from step 2
	// while replacing string/numeric literals with additional ? placeholders
	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sqlWithAnonymizedComments, &obfuscateSQLConfig, "")
	if err != nil {
		return "", err
	}

	return obfuscatedQuery.Query, nil
}
