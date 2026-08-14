// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cputicks // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/cputicks"

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

// Total returns the sum of all CPU tick fields.
// Guest and GuestNice are excluded because the Linux kernel already
// accounts them inside User and Nice respectively.
func (s Stat) Total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.Iowait +
		s.Irq + s.Softirq + s.Steal
}
