// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bytes"
	"encoding/xml"
	"strings"
	"unicode"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"go.uber.org/zap"
)

var xmlPlanObfuscationAttrs = []string{
	"StatementText",
	"ConstValue",
	"ScalarString",
	"ParameterCompiledValue",
}

type obfuscator struct {
	*obfuscate.Obfuscator
	logger *zap.Logger
}

func newObfuscator(logger *zap.Logger) *obfuscator {
	return &obfuscator{
		Obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				DBMS: "mssql",
				// ObfuscateAndNormalize routes obfuscation through the go-sqllexer
				// engine, which is more tolerant than the legacy tokenizer: it does
				// not error on statements that reduce to nothing after comments are
				// stripped (returning an empty result instead of "result is empty"),
				// so comment-only statements no longer spam error logs or drop the
				// row. It also normalizes the output (collapsing whitespace and
				// stripping comments/aliases), which yields more stable query
				// signatures across semantically identical statements.
				ObfuscationMode: obfuscate.ObfuscateAndNormalize,
			},
		}),
		logger: logger,
	}
}

// sanitizeSQL strips non-semantic Unicode format characters (Unicode category
// Cf, e.g. a zero-width space U+200B) that carry no SQL semantics. Under the
// ObfuscateAndNormalize engine these characters no longer cause a hard failure,
// but they would otherwise survive into the obfuscated output as garbled bytes
// and, worse, cause an otherwise-identical statement to obfuscate to a different
// string. Stripping them keeps the obfuscated text clean and ensures the query
// signature is stable regardless of stray invisible characters.
func sanitizeSQL(sql string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, sql)
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	obfuscatedQuery, err := o.ObfuscateSQLString(sanitizeSQL(sql))
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
			// The decoder resolves the plan's default namespace into the Name.Space
			// of every element, while the original xmlns declaration survives as a
			// plain attribute on the root. Re-encoding that as-is makes the encoder
			// declare the namespace again on the root -- producing a duplicate xmlns
			// attribute that fails validation with "Attribute xmlns redefined" -- and
			// re-declare it on every descendant. Clearing Name.Space (on EndElement
			// too, so the tags stay matched) leaves the root's original declaration to
			// pass through exactly once and keeps descendants free of redundant ones.
			elem.Name.Space = ""
			for i := range elem.Attr {
				for _, attrName := range xmlPlanObfuscationAttrs {
					if elem.Attr[i].Name.Local == attrName {
						if elem.Attr[i].Value == "" {
							continue
						}
						val, err := o.obfuscateSQLString(elem.Attr[i].Value)
						if err != nil {
							o.logger.Warn("Unable to obfuscate SQL statement in query plan, redacting attribute", zap.String("attr", attrName), zap.Error(err))
							elem.Attr[i].Value = "?"
							continue
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
			elem.Name.Space = ""
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
