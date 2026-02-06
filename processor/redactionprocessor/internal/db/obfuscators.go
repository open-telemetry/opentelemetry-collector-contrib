// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

import (
	"encoding/json"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"go.uber.org/zap"
)

type databaseObfuscator interface {
	Obfuscate(string) (string, error)
	ObfuscateWithSystem(string, string) (string, error)
	ObfuscateAttribute(string, string) (string, error)
	ShouldProcessAttribute(string) bool
	SupportsSystem(string) bool
}

type dbAttributes struct {
	attributes map[string]bool
	dbSystems  map[string]bool
}

func (d *dbAttributes) ShouldProcessAttribute(attributeKey string) bool {
	return d.attributes[attributeKey]
}

func (d *dbAttributes) SupportsSystem(dbSystem string) bool {
	if len(d.dbSystems) == 0 || dbSystem == "" {
		return false
	}
	return d.dbSystems[dbSystem]
}

type sqlObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &sqlObfuscator{}

func (o *sqlObfuscator) Obfuscate(s string) (string, error) {
	obfuscatedQuery, err := o.obfuscator.ObfuscateSQLString(s)
	if err != nil {
		return s, err
	}
	return obfuscatedQuery.Query, nil
}

func (o *sqlObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *sqlObfuscator) ObfuscateWithSystem(s, dbSystem string) (string, error) {
	obfuscatedQuery, err := o.obfuscator.ObfuscateSQLStringForDBMS(s, dbSystem)
	if err != nil {
		return s, err
	}
	return obfuscatedQuery.Query, nil
}

type redisObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &redisObfuscator{}

func (o *redisObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateRedisString(s), nil
}

func (o *redisObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *redisObfuscator) ObfuscateWithSystem(s, _ string) (string, error) {
	return o.Obfuscate(s)
}

type memcachedObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &memcachedObfuscator{}

func (o *memcachedObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateMemcachedString(s), nil
}

func (o *memcachedObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *memcachedObfuscator) ObfuscateWithSystem(s, _ string) (string, error) {
	return o.Obfuscate(s)
}

type mongoObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
	logger     *zap.Logger
}

var _ databaseObfuscator = &mongoObfuscator{}

func (o *mongoObfuscator) Obfuscate(s string) (string, error) {
	if !isValidJSON(s) {
		o.logger.Debug("mongo span name not obfuscated due to invalid JSON input")
		return s, nil
	}
	return o.obfuscator.ObfuscateMongoDBString(s), nil
}

func (o *mongoObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *mongoObfuscator) ObfuscateWithSystem(s, _ string) (string, error) {
	return o.Obfuscate(s)
}

type opensearchObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
	logger     *zap.Logger
}

var _ databaseObfuscator = &opensearchObfuscator{}

func (o *opensearchObfuscator) Obfuscate(s string) (string, error) {
	if !isValidJSON(s) {
		o.logger.Debug("opensearch span name not obfuscated due to invalid JSON input")
		return s, nil
	}
	return o.obfuscator.ObfuscateOpenSearchString(s), nil
}

func (o *opensearchObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *opensearchObfuscator) ObfuscateWithSystem(s, _ string) (string, error) {
	return o.Obfuscate(s)
}

type esObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
	logger     *zap.Logger
}

var _ databaseObfuscator = &esObfuscator{}

func (o *esObfuscator) Obfuscate(s string) (string, error) {
	if !isValidJSON(s) {
		o.logger.Debug("elasticsearch span name not obfuscated due to invalid JSON input")
		return s, nil
	}
	return o.obfuscator.ObfuscateElasticSearchString(s), nil
}

func (o *esObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

func (o *esObfuscator) ObfuscateWithSystem(s, _ string) (string, error) {
	return o.Obfuscate(s)
}

func isValidJSON(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	return json.Valid([]byte(value))
}
