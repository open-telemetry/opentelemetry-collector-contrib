// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

import "github.com/DataDog/datadog-agent/pkg/obfuscate"

type databaseObfuscator interface {
	Obfuscate(string) (string, error)
	ObfuscateAttribute(string, string) (string, error)
	ShouldProcessAttribute(string) bool
}

type dbAttributes struct {
	attributes map[string]bool
}

func (d *dbAttributes) ShouldProcessAttribute(attributeKey string) bool {
	return d.attributes[attributeKey]
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

type valkeyObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &valkeyObfuscator{}

func (o *valkeyObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateRedisString(s), nil
}

func (o *valkeyObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
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

type mongoObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &mongoObfuscator{}

func (o *mongoObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateMongoDBString(s), nil
}

func (o *mongoObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

type opensearchObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &opensearchObfuscator{}

func (o *opensearchObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateOpenSearchString(s), nil
}

func (o *opensearchObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}

type esObfuscator struct {
	dbAttributes
	obfuscator *obfuscate.Obfuscator
}

var _ databaseObfuscator = &esObfuscator{}

func (o *esObfuscator) Obfuscate(s string) (string, error) {
	return o.obfuscator.ObfuscateElasticSearchString(s), nil
}

func (o *esObfuscator) ObfuscateAttribute(attributeValue, attributeKey string) (string, error) {
	if !o.ShouldProcessAttribute(attributeKey) {
		return attributeValue, nil
	}
	return o.Obfuscate(attributeValue)
}
