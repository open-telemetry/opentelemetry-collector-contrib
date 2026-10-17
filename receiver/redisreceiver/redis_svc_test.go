// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newFakeAPIParser() *redisSvc {
	return newRedisSvc(fakeClient{}, zap.NewNop())
}

func TestParser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	s := newFakeAPIParser()
	info, err := s.info()
	require.NoError(t, err)
	require.Len(t, info, 146)
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check
}

func TestParser_StandaloneSkipsClusterInfo(t *testing.T) {
	client := &standaloneClient{}
	s := newRedisSvc(client, zap.NewNop())
	info, err := s.info()
	require.NoError(t, err)
	require.False(t, client.clusterInfoCalled, "CLUSTER INFO should not be attempted against a standalone server")
	require.Len(t, info, 1)
	require.Equal(t, "0", info["cluster_enabled"])
}
