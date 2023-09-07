package sampling

import (
	"io"
	"regexp"
	"strconv"
)

type OTelTraceState struct {
	commonTraceState

	// sampling r and t-values
	rnd       Randomness // r value parsed, as unsigned
	rvalue    string     // 14 ASCII hex digits
	threshold Threshold  // t value parsed, as a threshold
	tvalue    string     // 1-14 ASCII hex digits
}

const (
	// RName is the OTel tracestate field for R-value
	RName = "rv"
	// TName is the OTel tracestate field for T-value
	TName = "th"

	// hardMaxOTelLength is the maximum encoded size of an OTel
	// tracestate value.
	hardMaxOTelLength = 256

	// chr        = ucalpha / lcalpha / DIGIT / "." / "_" / "-"
	// ucalpha    = %x41-5A ; A-Z
	// lcalpha    = %x61-7A ; a-z
	// key        = lcalpha *(lcalpha / DIGIT )
	// value      = *(chr)
	// list-member = key ":" value
	// list        = list-member *( ";" list-member )
	otelKeyRegexp             = lcAlphaRegexp + lcDigitRegexp + `*`
	otelValueRegexp           = `[a-zA-Z0-9._\-]*`
	otelMemberRegexp          = `(?:` + otelKeyRegexp + `:` + otelValueRegexp + `)`
	otelSemicolonMemberRegexp = `(?:` + `;` + otelMemberRegexp + `)`
	otelTracestateRegexp      = `^` + otelMemberRegexp + otelSemicolonMemberRegexp + `*$`
)

var (
	otelTracestateRe = regexp.MustCompile(otelTracestateRegexp)

	otelSyntax = keyValueScanner{
		maxItems:  -1,
		trim:      false,
		separator: ';',
		equality:  ':',
	}
)

func NewOTelTraceState(input string) (otts OTelTraceState, _ error) {
	if len(input) > hardMaxOTelLength {
		return otts, ErrTraceStateSize
	}

	if !otelTracestateRe.MatchString(input) {
		return OTelTraceState{}, strconv.ErrSyntax
	}

	err := otelSyntax.scanKeyValues(input, func(key, value string) error {
		var err error
		switch key {
		case RName:
			if otts.rnd, err = RValueToRandomness(value); err == nil {
				otts.rvalue = value
			} else {
				// The zero-value for randomness implies always-sample;
				// the threshold test is R < T, but T is not meaningful
				// at zero, and this value implies zero adjusted count.
				otts.rnd = Randomness{}
			}
		case TName:
			if otts.threshold, err = TValueToThreshold(value); err == nil {
				otts.tvalue = value
			} else {
				otts.threshold = AlwaysSampleThreshold
			}
		default:
			otts.kvs = append(otts.kvs, KV{
				Key:   key,
				Value: value,
			})
		}
		return err
	})

	return otts, err
}

func (otts *OTelTraceState) HasRValue() bool {
	return otts.rvalue != ""
}

func (otts *OTelTraceState) RValue() string {
	return otts.rvalue
}

func (otts *OTelTraceState) RValueRandomness() Randomness {
	return otts.rnd
}

func (otts *OTelTraceState) HasTValue() bool {
	return otts.tvalue != ""
}

func (otts *OTelTraceState) HasNonZeroTValue() bool {
	return otts.HasTValue() && otts.TValueThreshold() != NeverSampleThreshold
}

func (otts *OTelTraceState) TValue() string {
	return otts.tvalue
}

func (otts *OTelTraceState) TValueThreshold() Threshold {
	return otts.threshold
}

func (otts *OTelTraceState) SetTValue(threshold Threshold, encoded string) {
	otts.threshold = threshold
	otts.tvalue = encoded
}

func (otts *OTelTraceState) UnsetTValue() {
	otts.tvalue = ""
	otts.threshold = Threshold{}
}

func (otts *OTelTraceState) SetRValue(randomness Randomness) {
	otts.rnd = randomness
	otts.rvalue = randomness.ToRValue()
}

func (otts *OTelTraceState) UnsetRValue() {
	otts.rvalue = ""
	otts.rnd = Randomness{}
}

func (otts *OTelTraceState) HasAnyValue() bool {
	return otts.HasRValue() || otts.HasTValue() || otts.HasExtraValues()
}

func (otts *OTelTraceState) Serialize(w io.StringWriter) {
	cnt := 0
	sep := func() {
		if cnt != 0 {
			w.WriteString(";")
		}
		cnt++
	}
	if otts.HasRValue() {
		sep()
		w.WriteString(RName)
		w.WriteString(":")
		w.WriteString(otts.RValue())
	}
	if otts.HasTValue() {
		sep()
		w.WriteString(TName)
		w.WriteString(":")
		w.WriteString(otts.TValue())
	}
	for _, kv := range otts.ExtraValues() {
		sep()
		w.WriteString(kv.Key)
		w.WriteString(":")
		w.WriteString(kv.Value)
	}
}
