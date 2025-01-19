// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialResolution(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	provided := []string{"endpoint-2", "endpoint-1"}
	res, err := newStaticResolver(provided, tb)
	require.NoError(t, err)

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// verify
	expected := []string{"endpoint-1", "endpoint-2"}
	assert.Equal(t, expected, resolved)
}

func TestResolvedOnlyOnce(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	expected := []string{"endpoint-1", "endpoint-2"}
	res, err := newStaticResolver(expected, tb)
	require.NoError(t, err)

	counter := 0
	res.onChange(func(_ []string) {
		counter++
	})

	// test
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()
	resolved, err := res.resolve(context.Background()) // second resolution, should be noop

	// verify
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
	assert.Equal(t, expected, resolved)
}

func TestFailOnMissingEndpoints(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	var expected []string

	// test
	res, err := newStaticResolver(expected, tb)

	// verify
	assert.Equal(t, errNoEndpoints, err)
	assert.Nil(t, res)
}
