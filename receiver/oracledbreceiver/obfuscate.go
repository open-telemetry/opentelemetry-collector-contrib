// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

// obfuscateSQLConfig obfuscates literals and normalizes the statement so that two
// textually different but semantically identical queries (differing only in
// whitespace, comments, or literal values) produce identical output. This
// canonical form is what downstream consumers hash into a stable query signature;
// with the previous obfuscate_only mode the original formatting was preserved, so
// the same Oracle sql_id could yield multiple query_text values (and signatures)
// purely due to formatting differences.
//
// KeepIdentifierQuotation is enabled so that a quoted identifier such as "a b" is
// not collapsed into the unquoted a b, which would otherwise be indistinguishable
// from an aliased column. Comments are stripped from the normalized text; the
// leading key=value comment tags are still captured separately from the raw SQL by
// sqlcomments.ExtractAndFilterComments (see scraper.go) before obfuscation runs and
// are emitted as db.query.comment_tags.
var obfuscateSQLConfig = obfuscate.SQLConfig{
	DBMS:                    "oracle",
	ObfuscationMode:         obfuscate.ObfuscateAndNormalize,
	KeepSQLAlias:            true,
	KeepBoolean:             true,
	KeepNull:                true,
	KeepIdentifierQuotation: true,
}

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		SQL: obfuscateSQLConfig,
	}))
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sql, &obfuscateSQLConfig, "")
	if err != nil {
		return "", err
	}

	return obfuscatedQuery.Query, nil
}
