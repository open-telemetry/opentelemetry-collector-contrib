package cardinality // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/cardinality"

import (
	"unicode/utf8"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

// Local configuration copies to avoid importing the parent package.
type LabelsCfg struct {
	MaxLabelsPerAlert   int
	MaxLabelValueLength int
	MaxTotalLabelSize   int
}
type SeriesCfg struct {
	MaxActiveSeries  int
	MaxSeriesPerRule int
}
type Config struct {
	Labels        LabelsCfg
	Allowlist     []string
	Blocklist     []string
	HashIfExceeds int
	HashAlgorithm string
	Series        SeriesCfg
	Enforcement   struct {
		Mode           string
		OverflowAction string
	}
}

type Limiter struct{ cfg Config }

func New(cfg Config) *Limiter { return &Limiter{cfg: cfg} }

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
		// blocklist wins
		if _, b := block[k]; b {
			continue
		}
		// allowlist mode (if provided)
		if len(l.cfg.Allowlist) > 0 {
			if _, ok := allow[k]; !ok {
				continue
			}
		}
		// per-value cap
		if l.cfg.Labels.MaxLabelValueLength > 0 {
			v = truncate(v, l.cfg.Labels.MaxLabelValueLength)
		}
		out[k] = v
	}
	// NOTE: you can add MaxLabelsPerAlert / MaxTotalLabelSize enforcement here if desired.
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
