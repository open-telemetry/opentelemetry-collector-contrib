// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

import (
	"strings"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	semconv138 "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
)

type Obfuscator struct {
	obfuscators []databaseObfuscator
	// Datadog obfuscators keep mutable parser state, so each pooled set is used by only one call at a time.
	obfuscatorsPool            sync.Pool
	processAttributesEnabled   bool
	logger                     *zap.Logger
	allowFallbackWithoutSystem bool
}

func createAttributes(attributes []string) map[string]bool {
	attributesMap := make(map[string]bool, len(attributes))
	for _, attr := range attributes {
		attributesMap[attr] = true
	}
	return attributesMap
}

func NewObfuscator(cfg DBSanitizerConfig, logger *zap.Logger) *Obfuscator {
	if logger == nil {
		logger = zap.NewNop()
	}
	obfuscateConfig := obfuscate.Config{
		SQL: obfuscate.SQLConfig{
			ReplaceDigits:    true,
			KeepSQLAlias:     true,
			DollarQuotedFunc: true,
			ObfuscationMode:  "obfuscate_only",
		},
		Redis: obfuscate.RedisConfig{
			Enabled:       cfg.RedisConfig.Enabled,
			RemoveAllArgs: true,
		},
		Valkey: obfuscate.ValkeyConfig{
			Enabled:       cfg.ValkeyConfig.Enabled,
			RemoveAllArgs: true,
		},
		Memcached: obfuscate.MemcachedConfig{
			Enabled:     cfg.MemcachedConfig.Enabled,
			KeepCommand: true,
		},
		Mongo:      obfuscate.JSONConfig{Enabled: cfg.MongoConfig.Enabled},
		OpenSearch: obfuscate.JSONConfig{Enabled: cfg.OpenSearchConfig.Enabled},
		ES:         obfuscate.JSONConfig{Enabled: cfg.ESConfig.Enabled},
	}

	newObfuscators := func() ([]databaseObfuscator, bool) {
		return newDatabaseObfuscators(cfg, logger, obfuscate.NewObfuscator(obfuscateConfig))
	}

	obfuscators, processAttributesEnabled := newObfuscators()

	return &Obfuscator{
		obfuscators: obfuscators,
		obfuscatorsPool: sync.Pool{
			New: func() any {
				obfuscators, _ := newObfuscators()
				return obfuscators
			},
		},
		processAttributesEnabled:   processAttributesEnabled,
		logger:                     logger,
		allowFallbackWithoutSystem: cfg.AllowFallbackWithoutSystem,
	}
}

func newDatabaseObfuscators(cfg DBSanitizerConfig, logger *zap.Logger, o *obfuscate.Obfuscator) ([]databaseObfuscator, bool) {
	var obfuscators []databaseObfuscator
	processAttributesEnabled := false

	if cfg.SQLConfig.Enabled {
		dbAttrs := newDBAttributes(cfg.SQLConfig.Attributes, []string{
			semconv138.DBSystemNameOtherSQL.Value.AsString(),
			semconv138.DBSystemNameMySQL.Value.AsString(),
			semconv138.DBSystemNamePostgreSQL.Value.AsString(),
			semconv138.DBSystemNameMariaDB.Value.AsString(),
			semconv138.DBSystemNameSQLite.Value.AsString(),
		})
		processAttributesEnabled = processAttributesEnabled || len(dbAttrs.attributes) > 0
		obfuscators = append(obfuscators, &sqlObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
		})
	}

	if cfg.RedisConfig.Enabled {
		dbAttrs := newDBAttributes(cfg.RedisConfig.Attributes, []string{
			semconv138.DBSystemNameRedis.Value.AsString(),
		})
		processAttributesEnabled = processAttributesEnabled || len(dbAttrs.attributes) > 0
		obfuscators = append(obfuscators, &redisObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
		})
	}

	if cfg.ValkeyConfig.Enabled {
		dbAttrs := newDBAttributes(cfg.ValkeyConfig.Attributes, []string{
			"valkey", // Not part of semantic conventions
		})
		processAttributesEnabled = processAttributesEnabled || len(dbAttrs.attributes) > 0
		obfuscators = append(obfuscators, &redisObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
		})
	}

	if cfg.MemcachedConfig.Enabled {
		dbAttrs := newDBAttributes(cfg.MemcachedConfig.Attributes, []string{
			semconv138.DBSystemNameMemcached.Value.AsString(),
		})
		processAttributesEnabled = processAttributesEnabled || len(dbAttrs.attributes) > 0
		obfuscators = append(obfuscators, &memcachedObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
		})
	}

	if cfg.MongoConfig.Enabled {
		dbAttrs := newDBAttributes(cfg.MongoConfig.Attributes, []string{
			semconv138.DBSystemNameMongoDB.Value.AsString(),
		})
		processAttributesEnabled = processAttributesEnabled || len(dbAttrs.attributes) > 0
		obfuscators = append(obfuscators, &mongoObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
			logger:       logger,
		})
	}

	if cfg.OpenSearchConfig.Enabled {
		dbAttrs := newDBAttributes([]string{}, []string{
			semconv138.DBSystemNameOpenSearch.Value.AsString(),
		})
		obfuscators = append(obfuscators, &opensearchObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
			logger:       logger,
		})
	}

	if cfg.ESConfig.Enabled {
		dbAttrs := newDBAttributes([]string{}, []string{
			semconv138.DBSystemNameElasticsearch.Value.AsString(),
		})
		obfuscators = append(obfuscators, &esObfuscator{
			dbAttributes: dbAttrs,
			obfuscator:   o,
			logger:       logger,
		})
	}

	return obfuscators, processAttributesEnabled
}

func (o *Obfuscator) Obfuscate(s string) (string, error) {
	if !o.HasObfuscators() {
		return s, nil
	}

	obfuscators := o.getObfuscators()
	defer o.putObfuscators(obfuscators)

	for _, obfuscator := range obfuscators {
		obfuscatedValue, err := obfuscator.Obfuscate(s)
		if err != nil {
			return s, err
		}
		s = obfuscatedValue
	}
	return s, nil
}

func (o *Obfuscator) ObfuscateAttribute(attributeValue, attributeKey, dbSystem string) (string, error) {
	if !o.HasSpecificAttributes() {
		return attributeValue, nil
	}

	obfuscators := o.getObfuscators()
	defer o.putObfuscators(obfuscators)

	if dbSystem == "" {
		if o.allowFallbackWithoutSystem {
			return obfuscateSequentially(obfuscators, attributeValue, attributeKey)
		}
		return attributeValue, nil
	}

	dbSystem = strings.ToLower(dbSystem)
	for _, obfuscator := range obfuscators {
		if !obfuscator.SupportsSystem(dbSystem) {
			continue
		}
		if !obfuscator.ShouldProcessAttribute(attributeKey) {
			continue
		}
		return obfuscator.ObfuscateAttribute(attributeValue, attributeKey)
	}

	return attributeValue, nil
}

func obfuscateSequentially(obfuscators []databaseObfuscator, attributeValue, attributeKey string) (string, error) {
	result := attributeValue
	for _, obfuscator := range obfuscators {
		if !obfuscator.ShouldProcessAttribute(attributeKey) {
			continue
		}
		obfuscatedValue, err := obfuscator.ObfuscateAttribute(result, attributeKey)
		if err != nil {
			return attributeValue, err
		}
		result = obfuscatedValue
	}
	return result, nil
}

func (o *Obfuscator) HasSpecificAttributes() bool {
	return o.processAttributesEnabled
}

func (o *Obfuscator) HasObfuscators() bool {
	return len(o.obfuscators) > 0
}

func (o *Obfuscator) ObfuscateWithSystem(val, dbSystem string) (string, error) {
	if !o.HasObfuscators() {
		return val, nil
	}
	if dbSystem == "" {
		return val, nil
	}

	obfuscators := o.getObfuscators()
	defer o.putObfuscators(obfuscators)

	lower := strings.ToLower(dbSystem)
	for _, obfuscator := range obfuscators {
		if !obfuscator.SupportsSystem(lower) {
			continue
		}
		return obfuscator.ObfuscateWithSystem(val, lower)
	}
	return val, nil
}

func (o *Obfuscator) getObfuscators() []databaseObfuscator {
	return o.obfuscatorsPool.Get().([]databaseObfuscator)
}

func (o *Obfuscator) putObfuscators(obfuscators []databaseObfuscator) {
	o.obfuscatorsPool.Put(obfuscators)
}

func createSystems(systems []string) map[string]bool {
	if len(systems) == 0 {
		return nil
	}
	systemsMap := make(map[string]bool, len(systems))
	for _, system := range systems {
		systemsMap[strings.ToLower(system)] = true
	}
	return systemsMap
}

func newDBAttributes(attributes, systems []string) dbAttributes {
	return dbAttributes{
		attributes: createAttributes(attributes),
		dbSystems:  createSystems(systems),
	}
}
