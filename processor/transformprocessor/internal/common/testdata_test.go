// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("host.name", "localhost")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("scope")
	span := ss.Spans().AppendEmpty()
	span.SetName("span")
	event := span.Events().AppendEmpty()
	event.SetName("event")
	return td
}

func newTestLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("host.name", "localhost")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("scope")
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("body")
	return ld
}

func newTestProfiles() pprofile.Profiles {
	pd := pprofile.NewProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("host.name", "localhost")
	sp := rp.ScopeProfiles().AppendEmpty()
	sp.Scope().SetName("scope")
	profile := sp.Profiles().AppendEmpty()
	profile.SetOriginalPayloadFormat("operationA")
	return pd
}

// newTestMetrics builds a Metrics containing one metric of every pmetric type,
// each with a single data point, and exemplars on the types that support them,
// so data-point and exemplar consumers exercise all of their type branches.
func newTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", "localhost")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")

	gauge := sm.Metrics().AppendEmpty()
	gauge.SetName("gauge")
	gaugeDP := gauge.SetEmptyGauge().DataPoints().AppendEmpty()
	gaugeDP.SetDoubleValue(1.0)
	gaugeDP.Exemplars().AppendEmpty().SetDoubleValue(1.0)

	sum := sm.Metrics().AppendEmpty()
	sum.SetName("sum")
	sumDP := sum.SetEmptySum().DataPoints().AppendEmpty()
	sumDP.SetDoubleValue(2.0)
	sumDP.Exemplars().AppendEmpty().SetDoubleValue(2.0)

	histogram := sm.Metrics().AppendEmpty()
	histogram.SetName("histogram")
	histogramDP := histogram.SetEmptyHistogram().DataPoints().AppendEmpty()
	histogramDP.SetCount(1)
	histogramDP.Exemplars().AppendEmpty().SetDoubleValue(3.0)

	expHistogram := sm.Metrics().AppendEmpty()
	expHistogram.SetName("exp_histogram")
	expHistogramDP := expHistogram.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	expHistogramDP.SetCount(1)
	expHistogramDP.Exemplars().AppendEmpty().SetDoubleValue(4.0)

	summary := sm.Metrics().AppendEmpty()
	summary.SetName("summary")
	summary.SetEmptySummary().DataPoints().AppendEmpty().SetCount(1)

	return md
}
