// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cputicks // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/cputicks"

import "context"

// Stat stores raw CPU tick counts for a single logical CPU.
// Field order matches /proc/stat columns.
type Stat struct {
	CPU       string
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	Iowait    uint64
	Irq       uint64
	Softirq   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
}

func (s Stat) Total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.Iowait +
		s.Irq + s.Softirq + s.Steal + s.Guest + s.GuestNice
}

// Reader reads per-CPU tick counts from the operating system.
type Reader interface {
	ReadAll(ctx context.Context) ([]Stat, error)
	TicksPerSecond() uint64
}
