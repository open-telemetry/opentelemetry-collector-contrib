// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// cachedDecision is the metadata retained for a previously sampled trace so
// that late-arriving spans receive the same annotations as the original batch.
type cachedDecision struct {
	ruleName  string
	threshold sampling.Threshold
}

// decisionCache holds two independent LRU caches: one tracks traces that have
// been sampled (with metadata needed to stamp late spans), the other tracks
// traces that were dropped (presence alone is enough). A nil inner cache means
// that side of the cache is disabled.
type decisionCache struct {
	sampled    *lru.Cache[pcommon.TraceID, cachedDecision]
	nonSampled *lru.Cache[pcommon.TraceID, struct{}]
}

func newDecisionCache(cfg DecisionCacheConfig) (*decisionCache, error) {
	c := &decisionCache{}
	if cfg.SampledCacheSize > 0 {
		s, err := lru.New[pcommon.TraceID, cachedDecision](cfg.SampledCacheSize)
		if err != nil {
			return nil, fmt.Errorf("sampled cache: %w", err)
		}
		c.sampled = s
	}
	if cfg.NonSampledCacheSize > 0 {
		ns, err := lru.New[pcommon.TraceID, struct{}](cfg.NonSampledCacheSize)
		if err != nil {
			return nil, fmt.Errorf("non-sampled cache: %w", err)
		}
		c.nonSampled = ns
	}
	return c, nil
}

// lookup checks both caches for a recorded decision. The first bool is whether
// the trace was sampled; the second is whether any record was found.
func (c *decisionCache) lookup(id pcommon.TraceID) (cachedDecision, bool, bool) {
	if c.sampled != nil {
		if md, ok := c.sampled.Get(id); ok {
			return md, true, true
		}
	}
	if c.nonSampled != nil {
		if _, ok := c.nonSampled.Get(id); ok {
			return cachedDecision{}, false, true
		}
	}
	return cachedDecision{}, false, false
}

func (c *decisionCache) recordSampled(id pcommon.TraceID, md cachedDecision) {
	if c.sampled != nil {
		c.sampled.Add(id, md)
	}
}

func (c *decisionCache) recordNotSampled(id pcommon.TraceID) {
	if c.nonSampled != nil {
		c.nonSampled.Add(id, struct{}{})
	}
}
