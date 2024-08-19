// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t,
		&Config{
			Traces: TracesConfig{
				IgnoreResources: []string{},
				TraceBuffer:     1000,
				BucketInterval:  10 * time.Second,
			},
		},
		cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
