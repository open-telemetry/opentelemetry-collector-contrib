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
	creationParams := exportertest.NewNopCreateSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateTracesExporter(context.Background(), creationParams, cfg)

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, exp)
}

func TestLogExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopCreateSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, exp)
}
