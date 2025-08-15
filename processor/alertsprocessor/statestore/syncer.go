// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statestore // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/statestore"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

type Store struct {
	cfg   interface{}
	log   *zap.Logger
	mu    sync.Mutex
	state map[string]alertState // key: signal|ruleID|fp
}

type alertState struct {
	Signal   string
	Active   bool
	Since    time.Time
	LastEval time.Time
	Labels   map[string]string
	For      time.Duration
}

type Transition struct {
	Signal      string
	RuleID      string
	Fingerprint string
	From        string
	To          string
	Labels      map[string]string
	At          time.Time
}

func New(_cfg interface{}, log *zap.Logger) *Store {
	return &Store{cfg: _cfg, log: log, state: map[string]alertState{}}
}

func (s *Store) key(signal, ruleID, fp string) string { return signal + "|" + ruleID + "|" + fp }

func (s *Store) Sync(_ context.Context, _ time.Time) { /* hook for external state */ }

func (s *Store) Apply(results []evaluation.Result, ts time.Time) []Transition {
	s.mu.Lock()
	defer s.mu.Unlock()
	var trans []Transition
	seen := map[string]bool{}

	for _, r := range results {
		for _, inst := range r.Instances {
			fp := inst.Fingerprint
			if fp == "" {
				fp = computeFP(inst.Labels)
			}
			k := s.key(r.Signal, r.Rule.ID, fp)
			seen[k] = true

			prev, ok := s.state[k]
			if !ok {
				prev = alertState{Signal: r.Signal, Labels: inst.Labels, For: r.Rule.For}
			}
			cur := prev
			cur.LastEval = ts
			cur.Labels = inst.Labels
			cur.For = r.Rule.For

			if inst.Active {
				if !prev.Active {
					if cur.Since.IsZero() {
						cur.Since = ts
					}
					if cur.For <= 0 || ts.Sub(cur.Since) >= cur.For {
						cur.Active = true
						trans = append(trans, Transition{Signal: r.Signal, RuleID: r.Rule.ID, Fingerprint: fp, From: stateStr(prev.Active), To: "firing", Labels: inst.Labels, At: ts})
					}
				} else {
					cur.Active = true
				}
			} else {
				if prev.Active {
					cur.Active = false
					trans = append(trans, Transition{Signal: r.Signal, RuleID: r.Rule.ID, Fingerprint: fp, From: "firing", To: "resolved", Labels: inst.Labels, At: ts})
				}
				cur.Since = time.Time{}
			}
			s.state[k] = cur
		}
	}

	for k, st := range s.state {
		if st.Active && !seen[k] {
			parts := strings.SplitN(k, "|", 3)
			trans = append(trans, Transition{
				Signal: parts[0], RuleID: parts[1], Fingerprint: parts[2],
				From: "firing", To: "resolved", Labels: st.Labels, At: ts,
			})
			st.Active = false
			s.state[k] = st
		}
	}
	return trans
}

func stateStr(active bool) string {
	if active {
		return "firing"
	}
	return "pending"
}

func computeFP(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(labels[k])
		b.WriteString(",")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}
