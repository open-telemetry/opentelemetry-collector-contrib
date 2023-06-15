// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"bufio"
	"sync"
	"time"
)

// FlusherConfig is a configuration of Flusher helper
type FlusherConfig struct {
	Period time.Duration `mapstructure:"force_flush_period"`
}

// NewFlusherConfig creates a default Flusher config
func NewFlusherConfig() FlusherConfig {
	return FlusherConfig{
		// Empty or `0s` means that we will never force flush
		Period: time.Millisecond * 500,
	}
}

// Build creates Flusher from configuration
func (c *FlusherConfig) Build() *Flusher {
	return &Flusher{
		lastDataChange:     time.Now(),
		forcePeriod:        c.Period,
		previousDataLength: 0,
	}
}

// Flusher keeps information about flush state
type Flusher struct {
	// forcePeriod defines time from last flush which should pass before setting force to true.
	// Never forces if forcePeriod is set to 0
	forcePeriod time.Duration

	// lastDataChange tracks date of last data change (including new data and flushes)
	lastDataChange time.Time

	// previousDataLength:
	// if previousDataLength = 0 - no new data have been received after flush
	// if previousDataLength > 0 - there is data which has not been flushed yet and it doesn't changed since lastDataChange
	previousDataLength int
	rwLock             sync.RWMutex
}

func (f *Flusher) UpdateDataChangeTime(length int) {
	f.rwLock.Lock()
	defer f.rwLock.Unlock()
	// Skip if length is greater than 0 and didn't changed
	if length > 0 && length == f.previousDataLength {
		return
	}

	// update internal properties with new values if data length changed
	// because it means that data is flowing and being processed
	f.previousDataLength = length
	f.lastDataChange = time.Now()
}

// Flushed reset data length
func (f *Flusher) Flushed() {
	f.UpdateDataChangeTime(0)
}

// ShouldFlush returns true if data should be forcefully flushed
func (f *Flusher) ShouldFlush() bool {
	f.rwLock.RLock()
	defer f.rwLock.RUnlock()
	// Returns true if there is f.forcePeriod after f.lastDataChange and data length is greater than 0
	return f.forcePeriod > 0 && time.Since(f.lastDataChange) > f.forcePeriod && f.previousDataLength > 0
}

func (f *Flusher) SplitFunc(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)

		// Return as it is in case of error
		if err != nil {
			return
		}

		// Return token
		if token != nil {
			// Inform flusher that we just flushed
			f.Flushed()
			return
		}

		// If there is no token, force flush eventually
		if f.ShouldFlush() {
			// Inform flusher that we just flushed
			f.Flushed()
			token = trimWhitespacesFunc(data)
			advance = len(data)
			return
		}

		// Inform flusher that we didn't flushed
		f.UpdateDataChangeTime(len(data))
		return
	}
}
