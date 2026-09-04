// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	stdnet "net"

	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/ucal"
)

func readNetworkLinkInfo(_ context.Context, device string) (ucal.LinkInfo, error) {
	iface, err := stdnet.InterfaceByName(device)
	if err != nil {
		return ucal.LinkInfo{}, err
	}

	row := windows.MibIfRow2{InterfaceIndex: uint32(iface.Index)}
	if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row); err != nil {
		return ucal.LinkInfo{}, err
	}

	return linkInfoFromWindowsInterface(iface.Flags, row.ReceiveLinkSpeed, row.TransmitLinkSpeed), nil
}

func linkInfoFromWindowsInterface(flags stdnet.Flags, receiveLinkSpeedBitsPerSecond, transmitLinkSpeedBitsPerSecond uint64) ucal.LinkInfo {
	return ucal.LinkInfo{
		CapacityBitsPerSecond: float64(receiveLinkSpeedBitsPerSecond) + float64(transmitLinkSpeedBitsPerSecond),
		Loopback:              flags&stdnet.FlagLoopback != 0,
	}
}
