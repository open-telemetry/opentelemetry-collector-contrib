// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetHostInfo(t *testing.T) {
	logger := zap.NewNop()

	hostInfo := GetHostInfo(logger)
	require.NotNil(t, hostInfo)

	osHostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, hostInfo.OS, osHostname)
}

func TestGetHostname(t *testing.T) {
	logger := zap.NewNop()

	hostInfoAll := &HostInfo{
		FQDN: "fqdn",
		OS:   "os",
	}
	assert.Equal(t, hostInfoAll.GetHostname(logger), "fqdn")

	hostInfoInvalid := &HostInfo{
		FQDN: "fqdn_invalid",
		OS:   "os",
	}
	assert.Equal(t, hostInfoInvalid.GetHostname(logger), "os")

	hostInfoMissingFQDN := &HostInfo{
		OS: "os",
	}
	assert.Equal(t, hostInfoMissingFQDN.GetHostname(logger), "os")

}
