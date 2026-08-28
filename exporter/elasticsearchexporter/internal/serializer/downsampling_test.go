// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownsampleEvent(t *testing.T) {
	type result struct {
		index string
		count uint16
	}

	var pushedData []result
	push := func(count uint16, index string) error {
		pushedData = append(pushedData, result{index, count})
		return nil
	}

	// To make the expected data deterministic, seed the random number generator.
	// If the seed changes or the random number generator changes, this test will fail.
	rnd = rand.New(rand.NewPCG(0, 0))

	err := DownsampleEvent(1000, ".otel-default", push)
	require.NoError(t, err)

	expectedData := []result{
		{"profiling-events-5pow01.otel-default", 201},
		{"profiling-events-5pow02.otel-default", 42},
		{"profiling-events-5pow03.otel-default", 9},
		{"profiling-events-5pow04.otel-default", 2},
		{"profiling-events-5pow05.otel-default", 1},
		{"profiling-events-5pow06.otel-default", 1},
	}

	require.Equal(t, expectedData, pushedData)
}

func TestDownsampleEventIndexFormat(t *testing.T) {
	var indices []string
	push := func(_ uint16, index string) error {
		indices = append(indices, index)
		return nil
	}

	err := DownsampleEvent(1000, "", push)
	require.NoError(t, err)

	assert.NotEmpty(t, indices)
	for _, index := range indices {
		assert.True(t, strings.HasPrefix(index, "profiling-events-5pow"), "unexpected index: %s", index)
		assert.False(t, strings.HasSuffix(index, ".otel-default"), "ecs mode index should not have .otel-default suffix: %s", index)
	}
}
