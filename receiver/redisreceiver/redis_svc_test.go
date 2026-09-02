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
	// 134 keys from INFO plus 12 keys from CLUSTER INFO.
	require.Len(t, info, 146)
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check from INFO
	require.Equal(t, "ok", info["cluster_state"])          // spot check from CLUSTER INFO
}

func TestParser_InfoError(t *testing.T) {
	s := newRedisSvc(erroringInfoClient{})
	info, err := s.info()
	require.Error(t, err)
	require.Nil(t, info)
}

func TestParser_ClusterInfoErrorIsBestEffort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	s := newRedisSvc(erroringClusterInfoClient{})
	info, err := s.info()
	require.NoError(t, err)
	// Only the 134 keys from INFO; CLUSTER INFO failed and was skipped rather than
	// failing the whole scrape.
	require.Len(t, info, 134)
	require.NotContains(t, info, "cluster_state")
}
