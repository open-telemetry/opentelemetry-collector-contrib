// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetUptime(t *testing.T) {
	svc := newRedisSvc(newFakeClient())
	info, _ := svc.info()
	uptime, err := info.getUptimeInSeconds()
	require.Nil(t, err)
	require.Equal(t, time.Duration(104946000000000), uptime)
}
