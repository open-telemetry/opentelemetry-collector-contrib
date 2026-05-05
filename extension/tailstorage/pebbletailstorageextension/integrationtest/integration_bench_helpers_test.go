// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type benchmarkShape struct {
	tracesPerBatch        int
	spansPerTrace         int
	payloadBytes          int
	parallelism           int
	targetReqPerSec       int
	decisionWait          time.Duration
	numTraces             uint64
	policy                benchmarkPolicy
	rootSpanDelay         time.Duration
	distributedTraceMode  bool
	require2xDecisionWait bool
}

func (s benchmarkShape) spansPerRequest() int {
	return s.tracesPerBatch * s.spansPerTrace
}

func (s benchmarkShape) requestsPerIteration() int {
	if s.distributedTraceMode {
		return 2
	}
	return 1
}

func (s benchmarkShape) payload() string {
	if s.payloadBytes <= 0 {
		return ""
	}
	return strings.Repeat("x", s.payloadBytes)
}

func (s benchmarkShape) validate(b *testing.B) {
	b.Helper()
	if s.tracesPerBatch <= 0 {
		b.Fatalf("tracesPerBatch must be > 0")
	}
	if s.spansPerTrace <= 0 {
		b.Fatalf("spansPerTrace must be > 0")
	}
	if s.distributedTraceMode && s.spansPerTrace < 2 {
		b.Fatalf("distributedTraceMode requires spansPerTrace >= 2")
	}
	if s.distributedTraceMode && s.rootSpanDelay < 0 {
		b.Fatalf("rootSpanDelay must be >= 0")
	}
	if s.policy.kind == benchmarkPolicyOTTLCondition && len(s.policy.ottlSpanConditions) == 0 {
		b.Fatalf("OTTL policy requires at least one span condition")
	}
	if s.policy.kind == benchmarkPolicyRootProbabilistic && (s.policy.samplingPercentage < 0 || s.policy.samplingPercentage > 100) {
		b.Fatalf("root probabilistic policy requires samplingPercentage in [0,100]")
	}
}

type benchmarkPolicyKind int

const (
	benchmarkPolicyProbabilistic benchmarkPolicyKind = iota
	benchmarkPolicyOTTLCondition
	benchmarkPolicyRootProbabilistic
)

type benchmarkPolicy struct {
	name                 string
	kind                 benchmarkPolicyKind
	samplingPercentage   float64
	ottlSpanConditions   []string
	backendNameInmemory  string
	backendNameWithStore string
}

func (p benchmarkPolicy) nameForBackend(backend string) string {
	if backend == "pebble" {
		if p.backendNameWithStore != "" {
			return p.backendNameWithStore
		}
		return p.name
	}
	if p.backendNameInmemory != "" {
		return p.backendNameInmemory
	}
	return p.name
}

func (p benchmarkPolicy) yaml() string {
	switch p.kind {
	case benchmarkPolicyProbabilistic:
		return fmt.Sprintf(`        type: probabilistic
        probabilistic:
          sampling_percentage: %.2f`, p.samplingPercentage)
	case benchmarkPolicyOTTLCondition:
		var spanConditions strings.Builder
		for _, cond := range p.ottlSpanConditions {
			spanConditions.WriteString(fmt.Sprintf("          - %q\n", cond))
		}
		return fmt.Sprintf(`        type: ottl_condition
        ottl_condition:
          error_mode: ignore
          span:
%s`, strings.TrimRight(spanConditions.String(), "\n"))
	case benchmarkPolicyRootProbabilistic:
		return fmt.Sprintf(`        type: and
        and:
          and_sub_policy:
            - name: root_span_only
              type: ottl_condition
              ottl_condition:
                error_mode: ignore
                span:
                  - "IsRootSpan()"
            - name: root_probabilistic
              type: probabilistic
              probabilistic:
                sampling_percentage: %.2f`, p.samplingPercentage)
	default:
		return `        type: always_sample`
	}
}

func benchmarkBatch(seed uint64, shape benchmarkShape) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	payload := shape.payload()
	spanSeq := seed*1_000_003 + 1

	for traceOffset := range shape.tracesPerBatch {
		traceID := uInt64ToTraceID(seed*65_537 + uint64(traceOffset))
		for spanOffset := range shape.spansPerTrace {
			span := ss.Spans().AppendEmpty()
			span.SetTraceID(traceID)
			span.SetSpanID(uInt64ToSpanID(spanSeq + uint64(spanOffset)))
			if payload != "" {
				span.Attributes().PutStr("payload", payload)
			}
		}
	}

	return td
}

func benchmarkDistributedTraceBatches(seed uint64, shape benchmarkShape) (ptrace.Traces, ptrace.Traces) {
	tdChildren := ptrace.NewTraces()
	rsChildren := tdChildren.ResourceSpans().AppendEmpty()
	ssChildren := rsChildren.ScopeSpans().AppendEmpty()

	tdRoots := ptrace.NewTraces()
	rsRoots := tdRoots.ResourceSpans().AppendEmpty()
	ssRoots := rsRoots.ScopeSpans().AppendEmpty()

	payload := shape.payload()
	spanSeq := seed*1_000_003 + 1

	for traceOffset := range shape.tracesPerBatch {
		traceID := uInt64ToTraceID(seed*65_537 + uint64(traceOffset))
		rootSpanID := uInt64ToSpanID(spanSeq)
		spanSeq++

		for childIdx := 0; childIdx < shape.spansPerTrace-1; childIdx++ {
			span := ssChildren.Spans().AppendEmpty()
			span.SetTraceID(traceID)
			span.SetSpanID(uInt64ToSpanID(spanSeq))
			span.SetParentSpanID(rootSpanID)
			if payload != "" {
				span.Attributes().PutStr("payload", payload)
			}
			spanSeq++
		}

		root := ssRoots.Spans().AppendEmpty()
		root.SetTraceID(traceID)
		root.SetSpanID(rootSpanID)
		if payload != "" {
			root.Attributes().PutStr("payload", payload)
		}
	}

	return tdChildren, tdRoots
}

func uInt64ToTraceID(v uint64) pcommon.TraceID {
	var id pcommon.TraceID
	for i := range 8 {
		id[i] = byte(v >> (8 * i))
		id[i+8] = byte((v + 0x9e3779b97f4a7c15) >> (8 * i))
	}
	return id
}

func uInt64ToSpanID(v uint64) pcommon.SpanID {
	var id pcommon.SpanID
	for i := range 8 {
		id[i] = byte(v >> (8 * i))
	}
	return id
}

func startRSSSampler(interval time.Duration) func() uint64 {
	var peak atomic.Uint64
	initial, err := currentRSSBytes()
	if err == nil {
		peak.Store(initial)
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rss, err := currentRSSBytes()
				if err != nil {
					continue
				}
				for {
					prev := peak.Load()
					if rss <= prev || peak.CompareAndSwap(prev, rss) {
						break
					}
				}
			case <-done:
				return
			}
		}
	}()

	return func() uint64 {
		close(done)
		rss, err := currentRSSBytes()
		if err == nil {
			for {
				prev := peak.Load()
				if rss <= prev || peak.CompareAndSwap(prev, rss) {
					break
				}
			}
		}
		return peak.Load()
	}
}

func currentRSSBytes() (uint64, error) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for line := range bytes.SplitSeq(status, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("VmRSS:")) {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected VmRSS format: %q", line)
		}
		kb, err := strconv.ParseUint(string(fields[1]), 10, 64)
		if err != nil {
			return 0, err
		}
		return kb * 1024, nil
	}
	return 0, errors.New("VmRSS not found in /proc/self/status")
}

func requireBenchTimeAtLeast(b *testing.B, minDuration time.Duration) {
	d, err := benchTimeDuration()
	if err != nil {
		b.Fatalf("invalid -test.benchtime: %v", err)
	}
	if d < minDuration {
		b.Fatalf("benchmark benchtime %v must be >= 2x decision_wait (%v); rerun with -benchtime >= %v", d, minDuration/2, minDuration)
	}
}

func benchTimeDuration() (time.Duration, error) {
	benchTime := "1s"
	if f := flag.Lookup("test.benchtime"); f != nil {
		benchTime = f.Value.String()
	}
	if strings.HasSuffix(benchTime, "x") {
		return 0, fmt.Errorf("benchtime %q must be time-based", benchTime)
	}
	d, err := time.ParseDuration(benchTime)
	if err != nil {
		return 0, fmt.Errorf("unable to parse -test.benchtime=%q: %w", benchTime, err)
	}
	return d, nil
}

func startFixedRateLimiter(targetReqPerSec int) (acquire, stop func()) {
	if targetReqPerSec <= 0 {
		return func() {}, func() {}
	}

	interval := time.Second / time.Duration(targetReqPerSec)
	if interval <= 0 {
		interval = time.Nanosecond
	}

	tokens := make(chan struct{}, targetReqPerSec)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				select {
				case tokens <- struct{}{}:
				default:
				}
			}
		}
	}()

	return func() { <-tokens }, func() { close(done) }
}

func dirSizeBytes(path string) (uint64, error) {
	if path == "" {
		return 0, nil
	}
	var total uint64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += uint64(info.Size())
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}
