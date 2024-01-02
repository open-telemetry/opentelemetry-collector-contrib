// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAvailableLocalAddress(t *testing.T) {
	addr := GetAvailableLocalAddress(t)

	// Endpoint should be free.
	ln0, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	require.NotNil(t, ln0)
	t.Cleanup(func() {
		require.NoError(t, ln0.Close())
	})

	// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
	ln1, err := net.Listen("tcp", addr)
	require.Error(t, err)
	require.Nil(t, ln1)
}
func TestGetAvailableLocalUDPAddress(t *testing.T) {
	addr := GetAvailableLocalNetworkAddress(t, "udp")
	// Endpoint should be free.
	ln0, err := net.ListenPacket("udp", addr)
	require.NoError(t, err)
	require.NotNil(t, ln0)
	t.Cleanup(func() {
		require.NoError(t, ln0.Close())
	})

	// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
	ln1, err := net.ListenPacket("udp", addr)
	require.Error(t, err)
	require.Nil(t, ln1)
}

func TestCreateExclusionsList(t *testing.T) {
	// Test two examples of typical output from "netsh interface ipv4 show excludedportrange protocol=tcp"
	emptyExclusionsText := `

Protocol tcp Port Exclusion Ranges

Start Port    End Port      
----------    --------      

* - Administered port exclusions.`

	exclusionsText := `

Start Port    End Port
----------    --------
     49697       49796
     49797       49896

* - Administered port exclusions.
`
	exclusions := createExclusionsList(t, exclusionsText)
	require.Equal(t, len(exclusions), 2)

	emptyExclusions := createExclusionsList(t, emptyExclusionsText)
	require.Equal(t, len(emptyExclusions), 0)
}
