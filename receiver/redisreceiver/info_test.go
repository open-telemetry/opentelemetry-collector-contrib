// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetUptime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	svc := newRedisSvc(newFakeClient())
	info, _ := svc.info()
	uptime, err := info.getUptimeInSeconds()
	require.NoError(t, err)
	require.Equal(t, time.Duration(104946000000000), uptime)
}
