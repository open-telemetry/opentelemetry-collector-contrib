// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/statestore"
)

func TestSeriesBuilder_BuildProducesMetrics(t *testing.T) {
	sb := NewSeriesBuilder(map[string]string{"group": "demo"})

	res := []evaluation.Result{{
		Rule:   evaluation.Rule{ID: "r1", Name: "R1", Signal: "logs"},
		Signal: "logs",
		Instances: []evaluation.Instance{{
			RuleID:      "r1",
			Fingerprint: "k",
			Active:      true,
			Value:       1,
			Labels:      map[string]string{"service.name": "svc"},
		}},
	}}

	trans := []statestore.Transition{
		{From: "inactive", To: "firing", Labels: map[string]string{"rule_id": "r1"}, At: time.Now()},
	}

	md := sb.Build(res, trans, time.Now())
	if md.DataPointCount() == 0 {
		t.Fatalf("expected synthetic metrics, got none")
	}

	// sanity: metric names exist
	names := collectMetricNames(md)
	if !contains(names, "otel_alert_state") {
		t.Fatalf("expected otel_alert_state in %v", names)
	}
	if !contains(names, "otel_alert_transitions_total") {
		t.Fatalf("expected otel_alert_transitions_total in %v", names)
	}
}

func collectMetricNames(md pmetric.Metrics) []string {
	var out []string
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				out = append(out, ms.At(k).Name())
			}
		}
	}
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
