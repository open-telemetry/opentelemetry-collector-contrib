// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cputicks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTotal(t *testing.T) {
	tick := Stat{
		User: 10, Nice: 20, System: 30, Idle: 40, Iowait: 5,
		Irq: 6, Softirq: 7, Steal: 8, Guest: 9, GuestNice: 1,
	}
	assert.Equal(t, uint64(126), tick.Total())
}

func TestTotal_Zero(t *testing.T) {
	assert.Equal(t, uint64(0), Stat{}.Total())
}
