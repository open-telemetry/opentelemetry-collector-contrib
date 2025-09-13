// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinality // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/cardinality"

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

type Control struct {
	MaxLabels     int
	MaxValLen     int
	MaxTotalSize  int
	HashIfExceeds int
	Allow         map[string]struct{}
	Block         map[string]struct{}
}

func (c *Control) Enforce(lbls map[string]string) map[string]string {
	if c == nil {
		return lbls
	}
	// blocklist
	for k := range lbls {
		if _, b := c.Block[k]; b {
			delete(lbls, k)
		}
	}
	// trim/hash long values and compute total
	total := 0
	for k, v := range lbls {
		if c.MaxValLen > 0 && len(v) > c.MaxValLen {
			if c.HashIfExceeds > 0 {
				sum := sha256.Sum256([]byte(v))
				lbls[k] = hex.EncodeToString(sum[:])[:8]
			} else {
				lbls[k] = v[:c.MaxValLen]
			}
		}
		total += len(k) + len(lbls[k])
	}
	// total size cap: drop non-allowed first
	if c.MaxTotalSize > 0 && total > c.MaxTotalSize {
		var losers []string
		for k := range lbls {
			if _, ok := c.Allow[k]; !ok {
				losers = append(losers, k)
			}
		}
		sort.Strings(losers)
		for _, k := range losers {
			if total <= c.MaxTotalSize {
				break
			}
			total -= len(k) + len(lbls[k])
			delete(lbls, k)
		}
	}
	// label count cap
	if c.MaxLabels > 0 && len(lbls) > c.MaxLabels {
		var keys []string
		for k := range lbls {
			if _, ok := c.Allow[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for len(lbls) > c.MaxLabels && len(keys) > 0 {
			k := keys[len(keys)-1]
			delete(lbls, k)
			keys = keys[:len(keys)-1]
		}
	}
	return lbls
}
