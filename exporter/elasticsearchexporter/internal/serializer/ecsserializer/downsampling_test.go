// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsserializer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"
)

func TestIndexDownsampledEvent(t *testing.T) {
	var indices []string
	pushData := func(_ any, _, index string) error {
		indices = append(indices, index)
		return nil
	}

	err := serializeprofiles.IndexDownsampledEvent(serializeprofiles.StackTraceEvent{Count: 1000}, "", pushData)
	require.NoError(t, err)

	assert.NotEmpty(t, indices)
	for _, index := range indices {
		assert.True(t, strings.HasPrefix(index, "profiling-events-5pow"), "unexpected index: %s", index)
		assert.False(t, strings.HasSuffix(index, ".otel-default"), "ecs mode index should not have .otel-default suffix: %s", index)
	}
}
