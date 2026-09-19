// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal

import (
	"errors"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNetworkUtilizationCalculator(t *testing.T) {
	now := pcommon.NewTimestampFromTime(time.Unix(100, 0))
	later := pcommon.NewTimestampFromTime(time.Unix(110, 0))

	calculator := &NetworkUtilizationCalculator{}
	results, hostUtilization, hasHostUtilization, err := calculator.Calculate(now, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 1000, BytesSent: 2000},
	}, func(string) (LinkInfo, error) {
		return LinkInfo{SpeedMbps: 100, FullDuplex: true}, nil
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "eth0", results[0].Device)
	assert.False(t, results[0].HasUtilization)
	assert.False(t, hasHostUtilization)
	assert.Zero(t, hostUtilization)

	results, hostUtilization, hasHostUtilization, err = calculator.Calculate(later, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 2000, BytesSent: 3000},
		{Name: "eth1", BytesRecv: 1000, BytesSent: 1000},
		{Name: "lo", BytesRecv: 1000, BytesSent: 1000},
	}, func(device string) (LinkInfo, error) {
		switch device {
		case "eth0":
			return LinkInfo{SpeedMbps: 100, FullDuplex: true}, nil
		case "eth1":
			return LinkInfo{}, nil
		case "lo":
			return LinkInfo{Loopback: true}, nil
		default:
			return LinkInfo{}, nil
		}
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "eth0", results[0].Device)
	assert.True(t, results[0].HasUtilization)
	assert.Equal(t, 0.000008, results[0].Utilization)
	assert.True(t, hasHostUtilization)
	assert.Equal(t, 0.000004, hostUtilization)
}

func TestNetworkUtilizationCalculatorSkipsReset(t *testing.T) {
	now := pcommon.NewTimestampFromTime(time.Unix(100, 0))
	later := pcommon.NewTimestampFromTime(time.Unix(110, 0))
	calculator := &NetworkUtilizationCalculator{}
	linkInfo := func(string) (LinkInfo, error) {
		return LinkInfo{SpeedMbps: 100}, nil
	}

	_, _, _, err := calculator.Calculate(now, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 1000, BytesSent: 1000},
	}, linkInfo)
	require.NoError(t, err)

	results, _, hasHostUtilization, err := calculator.Calculate(later, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 900, BytesSent: 1000},
	}, linkInfo)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].HasUtilization)
	assert.False(t, hasHostUtilization)
}

func TestNetworkUtilizationCalculatorUsesCapacityBitsPerSecond(t *testing.T) {
	now := pcommon.NewTimestampFromTime(time.Unix(100, 0))
	later := pcommon.NewTimestampFromTime(time.Unix(110, 0))
	calculator := &NetworkUtilizationCalculator{}
	linkInfo := func(string) (LinkInfo, error) {
		return LinkInfo{CapacityBitsPerSecond: 300_000_000}, nil
	}

	_, _, _, err := calculator.Calculate(now, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 1000, BytesSent: 2000},
	}, linkInfo)
	require.NoError(t, err)

	results, _, hasHostUtilization, err := calculator.Calculate(later, []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 2000, BytesSent: 3000},
	}, linkInfo)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].HasUtilization)
	assert.InDelta(t, 0.000005333, results[0].Utilization, 0.000000001)
	assert.True(t, hasHostUtilization)
}

func TestNetworkUtilizationCalculatorPropagatesLinkInfoError(t *testing.T) {
	calculator := &NetworkUtilizationCalculator{}
	_, _, _, err := calculator.Calculate(pcommon.NewTimestampFromTime(time.Unix(100, 0)), []net.IOCountersStat{
		{Name: "eth0", BytesRecv: 1000, BytesSent: 1000},
	}, func(string) (LinkInfo, error) {
		return LinkInfo{}, errors.New("link failed")
	})
	assert.EqualError(t, err, "link failed")
}
