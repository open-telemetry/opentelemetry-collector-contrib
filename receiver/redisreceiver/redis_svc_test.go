// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newFakeAPIParser() *redisSvc {
	return newRedisSvc(fakeClient{})
}

func TestParser(t *testing.T) {
	s := newFakeAPIParser()
	info, err := s.info()
	require.Nil(t, err)
	require.Equal(t, 130, len(info))
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check
}
