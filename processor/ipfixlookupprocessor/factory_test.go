// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), component.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotEqual(t, cfg, &Config{})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_ValidateConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.EqualError(t, component.ValidateConfig(cfg), "elasticsearch addresses must not be empty")
	cfg.(*Config).Elasticsearch.Connection.Addresses = []string{"http://localhost:9200"}
	assert.EqualError(t, component.ValidateConfig(cfg), "elasticsearch username must not be empty")
	cfg.(*Config).Elasticsearch.Connection.Username = "elastic"
	assert.EqualError(t, component.ValidateConfig(cfg), "elasticsearch password must not be empty")
	cfg.(*Config).Elasticsearch.Connection.Password = "changeme"
	assert.EqualError(t, component.ValidateConfig(cfg), "elasticsearch certificateFingerprint must not be empty")
	cfg.(*Config).Elasticsearch.Connection.CertificateFingerprint = "xxxx"
	assert.NoError(t, component.ValidateConfig(cfg), "elasticsearch addresses must not be empty")
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	conn, err := factory.CreateTracesProcessor(
		context.Background(),
		processortest.NewNopCreateSettings(),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
