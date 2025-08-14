package cardinality

import (
	"testing"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

func TestLimiter_FilterResult_Basic(t *testing.T) {
	lim := New(Config{
		Labels: LabelsCfg{
			MaxLabelsPerAlert:   0, // not enforced in limiter yet
			MaxLabelValueLength: 3, // will truncate
			MaxTotalLabelSize:   0,
		},
		Allowlist: []string{"a", "b"}, // only allow a,b if allowlist present
		Blocklist: []string{"drop"},   // drop is removed regardless
	})

	r := evaluation.Result{
		Rule:   evaluation.Rule{ID: "r1", Name: "R1"},
		Signal: "logs",
		Instances: []evaluation.Instance{{
			RuleID:      "r1",
			Fingerprint: "k",
			Active:      true,
			Value:       1,
			Labels: map[string]string{
				"a":    "hello", // -> "hel"
				"b":    "ok",    // stays
				"c":    "x",     // removed by allowlist
				"drop": "y",     // removed by blocklist
			},
		}},
	}

	out := lim.FilterResult(r)
	if len(out.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(out.Instances))
	}
	lbls := out.Instances[0].Labels
	if _, ok := lbls["drop"]; ok {
		t.Fatalf("label 'drop' should have been removed")
	}
	if _, ok := lbls["c"]; ok {
		t.Fatalf("label 'c' should be filtered by allowlist")
	}
	if got := lbls["a"]; got != "hel" {
		t.Fatalf("label 'a' not truncated: %q", got)
	}
	if got := lbls["b"]; got != "ok" {
		t.Fatalf("label 'b' changed: %q", got)
	}
}
