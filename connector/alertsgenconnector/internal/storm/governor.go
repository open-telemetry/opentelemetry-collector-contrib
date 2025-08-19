// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storm

import (
	"sync"
	"time"
	"go.uber.org/zap"
)

type Governor struct { rps, burst int; mu sync.Mutex; tokens float64; last time.Time; log *zap.Logger }

func NewGovernor(rps, burst int, log *zap.Logger) *Governor { if burst<1 { burst = 1 } ; return &Governor{ rps:rps, burst:burst, tokens:float64(burst), last:time.Now(), log:log } }

func (g *Governor) refillLocked(now time.Time) { if g.rps<=0 { g.tokens=float64(g.burst); g.last=now; return } ; el := now.Sub(g.last).Seconds(); g.tokens += el*float64(g.rps); if g.tokens>float64(g.burst) { g.tokens=float64(g.burst) } ; g.last = now }

func (g *Governor) Allow() bool { g.mu.Lock(); defer g.mu.Unlock(); now := time.Now(); g.refillLocked(now); if g.tokens>=1.0 { g.tokens -= 1.0; return true } ; return false }
