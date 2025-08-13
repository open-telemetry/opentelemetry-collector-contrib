// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t,
		&Config{
			Traces: datadogconfig.TracesConnectorConfig{
				TracesConfig: datadogconfig.TracesConfig{
					IgnoreResources:        []string{},
					PeerServiceAggregation: true,
					PeerTagsAggregation:    true,
					ComputeStatsBySpanKind: true,
				},
				TraceBuffer:    1000,
				BucketInterval: 10 * time.Second,
			},
		},
		cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
