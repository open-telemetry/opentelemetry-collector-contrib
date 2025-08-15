// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statestore

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

// Verifies that:
// 1) First appearance of an active instance -> To:"firing"
// 2) Disappearance on next evaluation           -> To:"resolved"
func TestStore_Apply_EmitsFiringAndResolved(t *testing.T) {
	// syncer.go: New(_cfg interface{}, log *zap.Logger) *Store
	// No concrete Config type; pass nil.
	st := New(nil, zap.NewNop())
	ts := time.Now()

	result := evaluation.Result{
		Rule:   evaluation.Rule{ID: "r1", Name: "r1", Signal: "logs"},
		Signal: "logs",
		Instances: []evaluation.Instance{{
			RuleID:      "r1",
			Fingerprint: "k1",
			Active:      true,
			Value:       1,
			Labels:      map[string]string{"rule_id": "r1"},
		}},
	}

	// First apply: expect a "firing" transition (From is "pending" per stateStr(false))
	tr1 := st.Apply([]evaluation.Result{result}, ts)
	if len(tr1) == 0 {
		t.Fatalf("expected at least one transition on first apply")
	}
	if tr1[0].To != "firing" {
		t.Fatalf("expected To=firing on first transition, got %q", tr1[0].To)
	}

	// Second apply with no instances: expect a "resolved" transition.
	tr2 := st.Apply(nil, ts.Add(time.Second))
	if len(tr2) == 0 {
		t.Fatalf("expected at least one transition when instance disappears")
	}
	if tr2[0].To != "resolved" {
		t.Fatalf("expected To=resolved on second transition, got %q", tr2[0].To)
	}
}
