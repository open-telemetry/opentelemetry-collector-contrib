// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestTracesExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateTraces(context.Background(), creationParams, cfg)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestLogExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateLogs(context.Background(), creationParams, cfg)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestOTLPConfigIsValid(t *testing.T) {
	// prepare
	factory := NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*Config)

	// test
	otlpCfg := defaultCfg.Protocol.OTLP

	// verify
	assert.NoError(t, otlpCfg.Validate())
}
