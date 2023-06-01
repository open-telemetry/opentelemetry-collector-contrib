package sampling

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
)

type OTelTraceState struct {
	commonTraceState

	// sampling r, s, and t-values
	ru uint64  // r value parsed, as unsigned
	r  string  // 14 ASCII hex digits
	sf float64 // s value parsed, as a probability
	s  string  // original float syntax preserved
	tf float64 // t value parsed, as a probability
	t  string  // original float syntax preserved
}

const (
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

	ErrRandomValueRange = fmt.Errorf("r-value out of range")

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
		case "r":
			var unsigned uint64
			unsigned, err = strconv.ParseUint(value, 16, 64)
			if err == nil {
				if unsigned >= 0x1p56 {
					err = ErrRandomValueRange
				} else {
					otts.r = value
					otts.ru = unsigned
				}
			}
		case "s":
			var prob float64
			prob, _, err = EncodedToProbabilityAndAdjustedCount(value)
			if err == nil {
				otts.s = value
				otts.sf = prob
			}
		case "t":
			var prob float64
			prob, _, err = EncodedToProbabilityAndAdjustedCount(value)
			if err == nil {
				otts.t = value
				otts.tf = prob
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

func (otts OTelTraceState) HasRValue() bool {
	return otts.r != ""
}

func (otts OTelTraceState) RValue() string {
	return otts.r
}

func (otts OTelTraceState) RValueUnsigned() uint64 {
	return otts.ru
}

func (otts OTelTraceState) HasSValue() bool {
	return otts.s != ""
}

func (otts OTelTraceState) SValue() string {
	return otts.s
}

func (otts OTelTraceState) SValueProbability() float64 {
	return otts.sf
}

func (otts OTelTraceState) HasTValue() bool {
	return otts.t != ""
}

func (otts OTelTraceState) TValue() string {
	return otts.t
}

func (otts OTelTraceState) TValueProbability() float64 {
	return otts.tf
}

func (otts OTelTraceState) HasAnyValue() bool {
	return otts.HasRValue() || otts.HasSValue() || otts.HasTValue() || otts.HasExtraValues()
}

func (otts OTelTraceState) Serialize(w io.StringWriter) {
	cnt := 0
	sep := func() {
		if cnt != 0 {
			w.WriteString(";")
		}
		cnt++
	}
	if otts.HasRValue() {
		sep()
		w.WriteString("r:")
		w.WriteString(otts.RValue())
	}
	if otts.HasSValue() {
		sep()
		w.WriteString("s:")
		w.WriteString(otts.SValue())
	}
	if otts.HasTValue() {
		sep()
		w.WriteString("t:")
		w.WriteString(otts.TValue())
	}
	for _, kv := range otts.ExtraValues() {
		sep()
		w.WriteString(kv.Key)
		w.WriteString(":")
		w.WriteString(kv.Value)
	}
}
