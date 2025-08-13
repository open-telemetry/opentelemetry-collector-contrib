package cardinality

import (
	"unicode/utf8"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

type Limiter struct {
	cfg alertsprocessor.CardinalityConfig
}

func New(cfg alertsprocessor.CardinalityConfig) *Limiter { return &Limiter{cfg: cfg} }

func (l *Limiter) FilterResult(r evaluation.Result) evaluation.Result {
	for i := range r.Instances {
		r.Instances[i].Labels = l.filterLabels(r.Instances[i].Labels)
	}
	return r
}

func (l *Limiter) filterLabels(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	allow := map[string]struct{}{}
	for _, k := range l.cfg.Allowlist {
		allow[k] = struct{}{}
	}
	block := map[string]struct{}{}
	for _, k := range l.cfg.Blocklist {
		block[k] = struct{}{}
	}
	for k, v := range in {
		if _, b := block[k]; b {
			continue
		}
		if len(l.cfg.Allowlist) > 0 {
			if _, ok := allow[k]; !ok {
				continue
			}
		}
		if l.cfg.Labels.MaxLabelValueLength > 0 {
			v = truncate(v, l.cfg.Labels.MaxLabelValueLength)
		}
		out[k] = v
	}
	return out
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}
