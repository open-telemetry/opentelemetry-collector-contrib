// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver"

import (
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type pingStats struct {
	minRtt   time.Duration
	avgRtt   time.Duration
	maxRtt   time.Duration
	lossRato float64
}

type pinger = interface {
	Run() error
	Stats() *pingStats
	IPString() string
	HostName() string
}

type defaultPinger struct {
	*probing.Pinger
}

func (p *defaultPinger) IPString() string {
	return p.IPAddr().IP.String()
}

func (p *defaultPinger) HostName() string {
	return p.Addr()
}

func (p *defaultPinger) Stats() *pingStats {
	return &pingStats{
		minRtt:   p.Statistics().MinRtt,
		avgRtt:   p.Statistics().AvgRtt,
		maxRtt:   p.Statistics().MaxRtt,
		lossRato: p.Statistics().PacketLoss,
	}
}

func defaultPingerFactory(target PingTarget) (pinger, error) {
	p, err := probing.NewPinger(target.Host)
	if err != nil {
		return nil, err
	}

	p.Interval = target.PingInterval
	p.Timeout = target.PingTimeout
	p.Count = target.PingCount

	return &defaultPinger{p}, nil
}
