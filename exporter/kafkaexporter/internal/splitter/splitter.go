// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package splitter provides size-aware splitting of pdata signal types so that
// each resulting chunk marshals to no more than a given byte limit.  It is used
// by the Kafka exporter to automatically split payloads that would otherwise
// exceed the producer MaxMessageBytes limit.
package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/splitter"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

// SplitTraces splits td into chunks such that every chunk marshals to at most
// maxBytes.  Splitting is attempted at three levels: ResourceSpans →
// ScopeSpans → individual Span.  If a single Span still exceeds maxBytes it
// is returned as-is; the caller must handle that case (the broker will reject
// it with MessageTooLarge).
func SplitTraces(td ptrace.Traces, m marshaler.TracesMarshaler, maxBytes int) ([]ptrace.Traces, error) {
	msgs, err := m.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	if fitsInLimit(msgs, maxBytes) {
		return []ptrace.Traces{td}, nil
	}

	// Split at ResourceSpans level.
	n := td.ResourceSpans().Len()
	if n > 1 {
		mid := n / 2
		left, right := ptrace.NewTraces(), ptrace.NewTraces()
		for i := range mid {
			td.ResourceSpans().At(i).CopyTo(left.ResourceSpans().AppendEmpty())
		}
		for i := mid; i < n; i++ {
			td.ResourceSpans().At(i).CopyTo(right.ResourceSpans().AppendEmpty())
		}
		lc, err := SplitTraces(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitTraces(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single ResourceSpans — split at ScopeSpans level.
	rs := td.ResourceSpans().At(0)
	nSS := rs.ScopeSpans().Len()
	if nSS > 1 {
		mid := nSS / 2
		left, right := ptrace.NewTraces(), ptrace.NewTraces()
		lRS, rRS := left.ResourceSpans().AppendEmpty(), right.ResourceSpans().AppendEmpty()
		rs.Resource().CopyTo(lRS.Resource())
		rs.Resource().CopyTo(rRS.Resource())
		lRS.SetSchemaUrl(rs.SchemaUrl())
		rRS.SetSchemaUrl(rs.SchemaUrl())
		for i := range mid {
			rs.ScopeSpans().At(i).CopyTo(lRS.ScopeSpans().AppendEmpty())
		}
		for i := mid; i < nSS; i++ {
			rs.ScopeSpans().At(i).CopyTo(rRS.ScopeSpans().AppendEmpty())
		}
		lc, err := SplitTraces(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitTraces(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single ScopeSpans — split at Span level.
	ss := rs.ScopeSpans().At(0)
	nSpans := ss.Spans().Len()
	if nSpans > 1 {
		mid := nSpans / 2
		left, right := ptrace.NewTraces(), ptrace.NewTraces()
		lRS, rRS := left.ResourceSpans().AppendEmpty(), right.ResourceSpans().AppendEmpty()
		rs.Resource().CopyTo(lRS.Resource())
		rs.Resource().CopyTo(rRS.Resource())
		lRS.SetSchemaUrl(rs.SchemaUrl())
		rRS.SetSchemaUrl(rs.SchemaUrl())
		lSS, rSS := lRS.ScopeSpans().AppendEmpty(), rRS.ScopeSpans().AppendEmpty()
		ss.Scope().CopyTo(lSS.Scope())
		ss.Scope().CopyTo(rSS.Scope())
		lSS.SetSchemaUrl(ss.SchemaUrl())
		rSS.SetSchemaUrl(ss.SchemaUrl())
		for i := range mid {
			ss.Spans().At(i).CopyTo(lSS.Spans().AppendEmpty())
		}
		for i := mid; i < nSpans; i++ {
			ss.Spans().At(i).CopyTo(rSS.Spans().AppendEmpty())
		}
		lc, err := SplitTraces(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitTraces(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single Span — cannot split further.
	return []ptrace.Traces{td}, nil
}

// SplitLogs splits ld into chunks such that every chunk marshals to at most
// maxBytes.  Splitting is attempted at three levels: ResourceLogs →
// ScopeLogs → individual LogRecord.  If a single LogRecord still exceeds
// maxBytes it is returned as-is.
func SplitLogs(ld plog.Logs, m marshaler.LogsMarshaler, maxBytes int) ([]plog.Logs, error) {
	msgs, err := m.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	if fitsInLimit(msgs, maxBytes) {
		return []plog.Logs{ld}, nil
	}

	n := ld.ResourceLogs().Len()
	if n > 1 {
		mid := n / 2
		left, right := plog.NewLogs(), plog.NewLogs()
		for i := range mid {
			ld.ResourceLogs().At(i).CopyTo(left.ResourceLogs().AppendEmpty())
		}
		for i := mid; i < n; i++ {
			ld.ResourceLogs().At(i).CopyTo(right.ResourceLogs().AppendEmpty())
		}
		lc, err := SplitLogs(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitLogs(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	rl := ld.ResourceLogs().At(0)
	nSL := rl.ScopeLogs().Len()
	if nSL > 1 {
		mid := nSL / 2
		left, right := plog.NewLogs(), plog.NewLogs()
		lRL, rRL := left.ResourceLogs().AppendEmpty(), right.ResourceLogs().AppendEmpty()
		rl.Resource().CopyTo(lRL.Resource())
		rl.Resource().CopyTo(rRL.Resource())
		lRL.SetSchemaUrl(rl.SchemaUrl())
		rRL.SetSchemaUrl(rl.SchemaUrl())
		for i := range mid {
			rl.ScopeLogs().At(i).CopyTo(lRL.ScopeLogs().AppendEmpty())
		}
		for i := mid; i < nSL; i++ {
			rl.ScopeLogs().At(i).CopyTo(rRL.ScopeLogs().AppendEmpty())
		}
		lc, err := SplitLogs(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitLogs(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	sl := rl.ScopeLogs().At(0)
	nLR := sl.LogRecords().Len()
	if nLR > 1 {
		mid := nLR / 2
		left, right := plog.NewLogs(), plog.NewLogs()
		lRL, rRL := left.ResourceLogs().AppendEmpty(), right.ResourceLogs().AppendEmpty()
		rl.Resource().CopyTo(lRL.Resource())
		rl.Resource().CopyTo(rRL.Resource())
		lRL.SetSchemaUrl(rl.SchemaUrl())
		rRL.SetSchemaUrl(rl.SchemaUrl())
		lSL, rSL := lRL.ScopeLogs().AppendEmpty(), rRL.ScopeLogs().AppendEmpty()
		sl.Scope().CopyTo(lSL.Scope())
		sl.Scope().CopyTo(rSL.Scope())
		lSL.SetSchemaUrl(sl.SchemaUrl())
		rSL.SetSchemaUrl(sl.SchemaUrl())
		for i := range mid {
			sl.LogRecords().At(i).CopyTo(lSL.LogRecords().AppendEmpty())
		}
		for i := mid; i < nLR; i++ {
			sl.LogRecords().At(i).CopyTo(rSL.LogRecords().AppendEmpty())
		}
		lc, err := SplitLogs(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitLogs(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single LogRecord — cannot split further.
	return []plog.Logs{ld}, nil
}

// SplitMetrics splits md into chunks such that every chunk marshals to at
// most maxBytes.  Splitting is attempted at three levels: ResourceMetrics →
// ScopeMetrics → individual Metric.  If a single Metric still exceeds
// maxBytes it is returned as-is.
func SplitMetrics(md pmetric.Metrics, m marshaler.MetricsMarshaler, maxBytes int) ([]pmetric.Metrics, error) {
	msgs, err := m.MarshalMetrics(md)
	if err != nil {
		return nil, err
	}
	if fitsInLimit(msgs, maxBytes) {
		return []pmetric.Metrics{md}, nil
	}

	n := md.ResourceMetrics().Len()
	if n > 1 {
		mid := n / 2
		left, right := pmetric.NewMetrics(), pmetric.NewMetrics()
		for i := range mid {
			md.ResourceMetrics().At(i).CopyTo(left.ResourceMetrics().AppendEmpty())
		}
		for i := mid; i < n; i++ {
			md.ResourceMetrics().At(i).CopyTo(right.ResourceMetrics().AppendEmpty())
		}
		lc, err := SplitMetrics(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitMetrics(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	rm := md.ResourceMetrics().At(0)
	nSM := rm.ScopeMetrics().Len()
	if nSM > 1 {
		mid := nSM / 2
		left, right := pmetric.NewMetrics(), pmetric.NewMetrics()
		lRM, rRM := left.ResourceMetrics().AppendEmpty(), right.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(lRM.Resource())
		rm.Resource().CopyTo(rRM.Resource())
		lRM.SetSchemaUrl(rm.SchemaUrl())
		rRM.SetSchemaUrl(rm.SchemaUrl())
		for i := range mid {
			rm.ScopeMetrics().At(i).CopyTo(lRM.ScopeMetrics().AppendEmpty())
		}
		for i := mid; i < nSM; i++ {
			rm.ScopeMetrics().At(i).CopyTo(rRM.ScopeMetrics().AppendEmpty())
		}
		lc, err := SplitMetrics(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitMetrics(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	sm := rm.ScopeMetrics().At(0)
	nM := sm.Metrics().Len()
	if nM > 1 {
		mid := nM / 2
		left, right := pmetric.NewMetrics(), pmetric.NewMetrics()
		lRM, rRM := left.ResourceMetrics().AppendEmpty(), right.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(lRM.Resource())
		rm.Resource().CopyTo(rRM.Resource())
		lRM.SetSchemaUrl(rm.SchemaUrl())
		rRM.SetSchemaUrl(rm.SchemaUrl())
		lSM, rSM := lRM.ScopeMetrics().AppendEmpty(), rRM.ScopeMetrics().AppendEmpty()
		sm.Scope().CopyTo(lSM.Scope())
		sm.Scope().CopyTo(rSM.Scope())
		lSM.SetSchemaUrl(sm.SchemaUrl())
		rSM.SetSchemaUrl(sm.SchemaUrl())
		for i := range mid {
			sm.Metrics().At(i).CopyTo(lSM.Metrics().AppendEmpty())
		}
		for i := mid; i < nM; i++ {
			sm.Metrics().At(i).CopyTo(rSM.Metrics().AppendEmpty())
		}
		lc, err := SplitMetrics(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitMetrics(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single Metric — cannot split further.
	return []pmetric.Metrics{md}, nil
}

// SplitProfiles splits pd into chunks such that every chunk marshals to at
// most maxBytes.  Splitting is attempted at three levels: ResourceProfiles →
// ScopeProfiles → individual Profile.  If a single Profile still exceeds
// maxBytes it is returned as-is.
func SplitProfiles(pd pprofile.Profiles, m marshaler.ProfilesMarshaler, maxBytes int) ([]pprofile.Profiles, error) {
	msgs, err := m.MarshalProfiles(pd)
	if err != nil {
		return nil, err
	}
	if fitsInLimit(msgs, maxBytes) {
		return []pprofile.Profiles{pd}, nil
	}

	n := pd.ResourceProfiles().Len()
	if n > 1 {
		mid := n / 2
		left, right := pprofile.NewProfiles(), pprofile.NewProfiles()
		for i := range mid {
			pd.ResourceProfiles().At(i).CopyTo(left.ResourceProfiles().AppendEmpty())
		}
		for i := mid; i < n; i++ {
			pd.ResourceProfiles().At(i).CopyTo(right.ResourceProfiles().AppendEmpty())
		}
		lc, err := SplitProfiles(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitProfiles(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	rp := pd.ResourceProfiles().At(0)
	nSP := rp.ScopeProfiles().Len()
	if nSP > 1 {
		mid := nSP / 2
		left, right := pprofile.NewProfiles(), pprofile.NewProfiles()
		lRP, rRP := left.ResourceProfiles().AppendEmpty(), right.ResourceProfiles().AppendEmpty()
		rp.Resource().CopyTo(lRP.Resource())
		rp.Resource().CopyTo(rRP.Resource())
		lRP.SetSchemaUrl(rp.SchemaUrl())
		rRP.SetSchemaUrl(rp.SchemaUrl())
		for i := range mid {
			rp.ScopeProfiles().At(i).CopyTo(lRP.ScopeProfiles().AppendEmpty())
		}
		for i := mid; i < nSP; i++ {
			rp.ScopeProfiles().At(i).CopyTo(rRP.ScopeProfiles().AppendEmpty())
		}
		lc, err := SplitProfiles(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitProfiles(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	sp := rp.ScopeProfiles().At(0)
	nP := sp.Profiles().Len()
	if nP > 1 {
		mid := nP / 2
		left, right := pprofile.NewProfiles(), pprofile.NewProfiles()
		lRP, rRP := left.ResourceProfiles().AppendEmpty(), right.ResourceProfiles().AppendEmpty()
		rp.Resource().CopyTo(lRP.Resource())
		rp.Resource().CopyTo(rRP.Resource())
		lRP.SetSchemaUrl(rp.SchemaUrl())
		rRP.SetSchemaUrl(rp.SchemaUrl())
		lSP, rSP := lRP.ScopeProfiles().AppendEmpty(), rRP.ScopeProfiles().AppendEmpty()
		sp.Scope().CopyTo(lSP.Scope())
		sp.Scope().CopyTo(rSP.Scope())
		lSP.SetSchemaUrl(sp.SchemaUrl())
		rSP.SetSchemaUrl(sp.SchemaUrl())
		for i := range mid {
			sp.Profiles().At(i).CopyTo(lSP.Profiles().AppendEmpty())
		}
		for i := mid; i < nP; i++ {
			sp.Profiles().At(i).CopyTo(rSP.Profiles().AppendEmpty())
		}
		lc, err := SplitProfiles(left, m, maxBytes)
		if err != nil {
			return nil, err
		}
		rc, err := SplitProfiles(right, m, maxBytes)
		if err != nil {
			return nil, err
		}
		return append(lc, rc...), nil
	}

	// Single Profile — cannot split further.
	return []pprofile.Profiles{pd}, nil
}

func fitsInLimit(msgs []marshaler.Message, maxBytes int) bool {
	for _, msg := range msgs {
		if len(msg.Value) > maxBytes {
			return false
		}
	}
	return true
}
