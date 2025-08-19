// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"time"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Transition struct { Signal, RuleID, Fingerprint, From, To, Reason, Severity string; Labels map[string]string; At time.Time }

type Engine interface { EvaluateTraces(ptrace.Traces, interface{}) []Transition; EvaluateMetrics(pmetric.Metrics, interface{}) []Transition }

type noop struct{}

func NewFromFile(_ string) (Engine, error) { return &noop{}, nil }

func (n *noop) EvaluateTraces(_ ptrace.Traces, _ interface{}) []Transition { return nil }

func (n *noop) EvaluateMetrics(_ pmetric.Metrics, _ interface{}) []Transition { return nil }
