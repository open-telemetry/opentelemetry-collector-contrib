// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"strings"
	"unicode"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

var xmlPlanObfuscationAttrs = []string{
	"StatementText",
	"ConstValue",
	"ScalarString",
	"ParameterCompiledValue",
}

const (
	xmlPlanCacheEntries   = 300
	maxCachedXMLPlanBytes = 256 * 1024
)

type obfuscator struct {
	*obfuscate.Obfuscator
	logger *zap.Logger

	xmlPlanCache  *lru.Cache[[sha256.Size]byte, string]
	cacheRequests metric.Int64Counter
	cacheHit      metric.AddOption
	cacheMiss     metric.AddOption
	cacheBypass   metric.AddOption
}

func newObfuscator(logger *zap.Logger, cacheEnabled bool) *obfuscator {
	var cache *lru.Cache[[sha256.Size]byte, string]
	if cacheEnabled {
		// The fixed positive size cannot make lru.New fail.
		cache, _ = lru.New[[sha256.Size]byte, string](xmlPlanCacheEntries)
	}
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
		logger:       logger,
		xmlPlanCache: cache,
	}
}

func (o *obfuscator) initCacheMetrics(settings component.TelemetrySettings, id component.ID) {
	if settings.MeterProvider == nil {
		return
	}
	telemetry, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		o.logger.Warn("Unable to create XML plan cache metrics", zap.Error(err))
		return
	}
	o.cacheRequests = telemetry.SqlserverXMLPlanCacheRequests
	o.cacheHit = metric.WithAttributes(attribute.String("receiver", id.String()), attribute.String("outcome", "hit"))
	o.cacheMiss = metric.WithAttributes(attribute.String("receiver", id.String()), attribute.String("outcome", "miss"))
	o.cacheBypass = metric.WithAttributes(attribute.String("receiver", id.String()), attribute.String("outcome", "bypass"))
}

func (o *obfuscator) recordCacheEvent(outcome metric.AddOption) {
	if o.cacheRequests != nil {
		o.cacheRequests.Add(context.Background(), 1, outcome)
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
	cacheable := o.xmlPlanCache != nil && len(rawPlan) <= maxCachedXMLPlanBytes
	var digest [sha256.Size]byte
	if cacheable {
		digest = sha256.Sum256([]byte(rawPlan))
		cached, ok := o.xmlPlanCache.Get(digest)
		if ok {
			o.recordCacheEvent(o.cacheHit)
			return cached, nil
		}
		o.recordCacheEvent(o.cacheMiss)
	} else if o.xmlPlanCache != nil {
		o.recordCacheEvent(o.cacheBypass)
	}

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

	result := buffer.String()
	if cacheable && result != "" && len(result) <= maxCachedXMLPlanBytes {
		o.xmlPlanCache.Add(digest, result)
	}
	return result, nil
}
