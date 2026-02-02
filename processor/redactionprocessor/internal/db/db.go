// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

type Obfuscator struct {
	obfuscators              []databaseObfuscator
	processAttributesEnabled bool
}

func createAttributes(attributes []string) map[string]bool {
	attributesMap := make(map[string]bool, len(attributes))
	for _, attr := range attributes {
		attributesMap[attr] = true
	}
	return attributesMap
}

func NewObfuscator(cfg DBSanitizerConfig) *Obfuscator {
	o := obfuscate.NewObfuscator(obfuscate.Config{
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
	})

	var obfuscators []databaseObfuscator
	processAttributesEnabled := false

	if cfg.SQLConfig.Enabled {
		attributes := createAttributes(cfg.SQLConfig.Attributes)
		processAttributesEnabled = processAttributesEnabled || len(attributes) > 0
		obfuscators = append(obfuscators, &sqlObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.RedisConfig.Enabled {
		attributes := createAttributes(cfg.RedisConfig.Attributes)
		processAttributesEnabled = processAttributesEnabled || len(attributes) > 0
		obfuscators = append(obfuscators, &redisObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.ValkeyConfig.Enabled {
		attributes := createAttributes(cfg.ValkeyConfig.Attributes)
		processAttributesEnabled = processAttributesEnabled || len(attributes) > 0
		obfuscators = append(obfuscators, &valkeyObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.MemcachedConfig.Enabled {
		attributes := createAttributes(cfg.MemcachedConfig.Attributes)
		processAttributesEnabled = processAttributesEnabled || len(attributes) > 0
		obfuscators = append(obfuscators, &memcachedObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.MongoConfig.Enabled {
		attributes := createAttributes(cfg.MongoConfig.Attributes)
		processAttributesEnabled = processAttributesEnabled || len(attributes) > 0
		obfuscators = append(obfuscators, &mongoObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.OpenSearchConfig.Enabled {
		attributes := createAttributes([]string{})
		obfuscators = append(obfuscators, &opensearchObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	if cfg.ESConfig.Enabled {
		attributes := createAttributes([]string{})
		obfuscators = append(obfuscators, &esObfuscator{
			dbAttributes: dbAttributes{
				attributes: attributes,
			},
			obfuscator: o,
		})
	}

	return &Obfuscator{
		obfuscators:              obfuscators,
		processAttributesEnabled: processAttributesEnabled,
	}
}

func (o *Obfuscator) Obfuscate(s string) (string, error) {
	for _, obfuscator := range o.obfuscators {
		obfuscatedValue, err := obfuscator.Obfuscate(s)
		if err != nil {
			return s, err
		}
		s = obfuscatedValue
	}
	return s, nil
}

func (o *Obfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.HasSpecificAttributes() {
		return attributeValue, nil
	}
	for _, obfuscator := range o.obfuscators {
		obfuscatedValue, err := obfuscator.ObfuscateAttribute(attributeValue, attributeKey)
		if err != nil {
			return attributeValue, err
		}
		attributeValue = obfuscatedValue
	}
	return attributeValue, nil
}

func (o *Obfuscator) HasSpecificAttributes() bool {
	return o.processAttributesEnabled
}

func (o *Obfuscator) HasObfuscators() bool {
	return len(o.obfuscators) > 0
}
