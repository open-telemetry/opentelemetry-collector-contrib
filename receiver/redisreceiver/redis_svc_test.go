// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func newFakeAPIParser() *redisSvc {
	return newRedisSvc(fakeClient{})
}

func TestParser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	s := newFakeAPIParser()
	info, err := s.info()
	require.NoError(t, err)
	require.Len(t, info, 134)
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check
}

func TestClusterParser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	s := newFakeAPIParser()
	clusterInfo, err := s.clusterInfo()
	require.NoError(t, err)
	require.Len(t, clusterInfo, 13)
	require.Equal(t, "ok", clusterInfo["cluster_state"])
	require.Equal(t, "16384", clusterInfo["cluster_slots_assigned"])
	require.Equal(t, "3", clusterInfo["cluster_size"]) // spot check: not "node_count"
}
