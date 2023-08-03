// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSetGetCPUCapacity(t *testing.T) {
	nodeInfo := newNodeInfo(zap.NewNop())
	nodeInfo.setCPUCapacity(int(4))
	assert.Equal(t, uint64(4), nodeInfo.getCPUCapacity())

	nodeInfo.setCPUCapacity(int32(2))
	assert.Equal(t, uint64(2), nodeInfo.getCPUCapacity())

	nodeInfo.setCPUCapacity(int64(4))
	assert.Equal(t, uint64(4), nodeInfo.getCPUCapacity())

	nodeInfo.setCPUCapacity(uint(2))
	assert.Equal(t, uint64(2), nodeInfo.getCPUCapacity())

	nodeInfo.setCPUCapacity(uint32(4))
	assert.Equal(t, uint64(4), nodeInfo.getCPUCapacity())

	nodeInfo.setCPUCapacity(uint64(2))
	assert.Equal(t, uint64(2), nodeInfo.getCPUCapacity())

	// with invalid type
	nodeInfo.setCPUCapacity("2")
	assert.Equal(t, uint64(0), nodeInfo.getCPUCapacity())

	// with negative value
	nodeInfo.setCPUCapacity(int64(-2))
	assert.Equal(t, uint64(0), nodeInfo.getCPUCapacity())
	nodeInfo.setCPUCapacity(int(-3))
	assert.Equal(t, uint64(0), nodeInfo.getCPUCapacity())
	nodeInfo.setCPUCapacity(int32(-4))
	assert.Equal(t, uint64(0), nodeInfo.getCPUCapacity())
}

func TestSetGetMemCapacity(t *testing.T) {
	nodeInfo := newNodeInfo(zap.NewNop())
	nodeInfo.setMemCapacity(int(2048))
	assert.Equal(t, uint64(2048), nodeInfo.getMemCapacity())

	nodeInfo.setMemCapacity(int32(1024))
	assert.Equal(t, uint64(1024), nodeInfo.getMemCapacity())

	nodeInfo.setMemCapacity(int64(2048))
	assert.Equal(t, uint64(2048), nodeInfo.getMemCapacity())

	nodeInfo.setMemCapacity(uint(1024))
	assert.Equal(t, uint64(1024), nodeInfo.getMemCapacity())

	nodeInfo.setMemCapacity(uint32(2048))
	assert.Equal(t, uint64(2048), nodeInfo.getMemCapacity())

	nodeInfo.setMemCapacity(uint64(1024))
	assert.Equal(t, uint64(1024), nodeInfo.getMemCapacity())

	// with invalid type
	nodeInfo.setMemCapacity("2")
	assert.Equal(t, uint64(0), nodeInfo.getMemCapacity())

	// with negative value
	nodeInfo.setMemCapacity(int64(-2))
	assert.Equal(t, uint64(0), nodeInfo.getMemCapacity())
}
