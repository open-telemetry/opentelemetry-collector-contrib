// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bytes"
	"encoding/xml"
	"strings"

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
			},
		}),
		logger: logger,
	}
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	obfuscatedQuery, err := o.ObfuscateSQLString(sql)
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
							o.logger.Warn("Unable to obfuscate SQL statement in query plan, redacting attribute", zap.Error(err))
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
