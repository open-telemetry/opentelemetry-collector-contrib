// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storm

import (
	"testing"
	"time"
	"go.uber.org/zap"
)

func TestBurstThenThrottle(t *testing.T) { g := NewGovernor(10, 3, zap.NewNop()); for i:=0;i<3;i++ { if !g.Allow() { t.Fatalf("expect allow %d", i) } } ; if g.Allow() { t.Fatal("expected throttle after burst") } ; time.Sleep(120*time.Millisecond); if !g.Allow() { t.Fatal("expected allow after refill") } }
