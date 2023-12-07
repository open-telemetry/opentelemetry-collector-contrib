// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"fmt"
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

	// ErrInconsistentSampling is returned when a sampler update
	// is illogical.  It is safe to ignore.  Samplers should avoid
	// this condition using a ThresholdLessThan() test.
	ErrInconsistentSampling = fmt.Errorf("cannot raise existing sampling probability")
	ErrInconsistentZero     = fmt.Errorf("cannot zero sampling probability")
)

func NewOTelTraceState(input string) (OTelTraceState, error) {
	// Note: the default value has threshold == 0 and tvalue == "".
	// It is important to recognize this as always-sample, meaning
	// to check HasTValue() before using TValueThreshold(), since
	// TValueThreshold() == NeverSampleThreshold when !HasTValue().
	otts := OTelTraceState{}

	if len(input) > hardMaxOTelLength {
		return otts, ErrTraceStateSize
	}

	if !otelTracestateRe.MatchString(input) {
		return otts, strconv.ErrSyntax
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
				otts.rvalue = ""
				otts.rnd = Randomness{}
			}
		case TName:
			if otts.threshold, err = TValueToThreshold(value); err == nil {
				otts.tvalue = value
			} else {
				otts.tvalue = ""
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

func (otts *OTelTraceState) TValue() string {
	return otts.tvalue
}

func (otts *OTelTraceState) TValueThreshold() Threshold {
	return otts.threshold
}

func (otts *OTelTraceState) UpdateTValueWithSampling(sampledThreshold Threshold, encodedTValue string) error {
	if otts.HasTValue() && ThresholdGreater(otts.threshold, sampledThreshold) {
		return ErrInconsistentSampling
	}
	otts.threshold = sampledThreshold
	otts.tvalue = encodedTValue
	return nil
}

func (otts *OTelTraceState) AdjustedCount() float64 {
	if !otts.HasTValue() {
		return 0
	}
	return 1.0 / otts.threshold.Probability()
}

func (otts *OTelTraceState) ClearTValue() {
	otts.tvalue = ""
	otts.threshold = Threshold{}
}

func (otts *OTelTraceState) SetRValue(randomness Randomness) {
	otts.rnd = randomness
	otts.rvalue = randomness.RValue()
}

func (otts *OTelTraceState) ClearRValue() {
	otts.rvalue = ""
	otts.rnd = Randomness{}
}

func (otts *OTelTraceState) HasAnyValue() bool {
	return otts.HasRValue() || otts.HasTValue() || otts.HasExtraValues()
}

func (otts *OTelTraceState) Serialize(w io.StringWriter) error {
	ser := serializer{writer: w}
	cnt := 0
	sep := func() {
		if cnt != 0 {
			ser.write(";")
		}
		cnt++
	}
	if otts.HasRValue() {
		sep()
		ser.write(RName)
		ser.write(":")
		ser.write(otts.RValue())
	}
	if otts.HasTValue() {
		sep()
		ser.write(TName)
		ser.write(":")
		ser.write(otts.TValue())
	}
	for _, kv := range otts.ExtraValues() {
		sep()
		ser.write(kv.Key)
		ser.write(":")
		ser.write(kv.Value)
	}
	return ser.err
}
