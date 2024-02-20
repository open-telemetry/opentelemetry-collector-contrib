// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"errors"
	"io"
	"regexp"
	"strconv"
)

// OpenTelemetryTraceState represents the `ot` section of the W3C tracestate
// which is specified generically in https://opentelemetry.io/docs/specs/otel/trace/tracestate-handling/.
//
// OpenTelemetry defines two specific values that convey sampling
// probability, known as T-Value (with "th", for threshold), R-Value
// (with key "rv", for random value), and extra values.
type OpenTelemetryTraceState struct {
	commonTraceState

	// sampling r and t-values
	rnd       Randomness // r value parsed, as unsigned
	rvalue    string     // 14 ASCII hex digits
	threshold Threshold  // t value parsed, as a threshold
	tvalue    string     // 1-14 ASCII hex digits
}

const (
	// rValueFieldName is the OTel tracestate field for R-value
	rValueFieldName = "rv"
	// tValueFieldName is the OTel tracestate field for T-value
	tValueFieldName = "th"

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
	otelKeyRegexp             = lcAlphaRegexp + lcAlphanumRegexp + `*`
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
	// is illogical, indicating that the tracestate was not
	// modified.  Preferably, Samplers will avoid seeing this
	// error by using a ThresholdGreater() test, which allows them
	// to report a more clear error to the user.  For example, if
	// data arrives sampled at 1/100 and an equalizing sampler is
	// configured for 1/2 sampling, the Sampler may detect the
	// illogical condition itself using ThresholdGreater and skip
	// the call to UpdateTValueWithSampling, which will have no
	// effect and return this error.  How a sampler decides to
	// handle this condition is up to the sampler: for example the
	// equalizing sampler can decide to pass through a span
	// indicating 1/100 sampling or it can reject the span.
	ErrInconsistentSampling = errors.New("cannot raise existing sampling probability")
)

// NewOpenTelemetryTraceState returns a parsed representation of the
// OpenTelemetry tracestate section.  Errors indicate an invalid
// tracestate was received.
func NewOpenTelemetryTraceState(input string) (OpenTelemetryTraceState, error) {
	otts := OpenTelemetryTraceState{}

	if len(input) > hardMaxOTelLength {
		return otts, ErrTraceStateSize
	}

	if !otelTracestateRe.MatchString(input) {
		return otts, strconv.ErrSyntax
	}

	err := otelSyntax.scanKeyValues(input, func(key, value string) error {
		var err error
		switch key {
		case rValueFieldName:
			if otts.rnd, err = RValueToRandomness(value); err == nil {
				otts.rvalue = value
			} else {
				// The zero-value for randomness implies always-sample;
				// the threshold test is R < T, but T is not meaningful
				// at zero, and this value implies zero adjusted count.
				otts.rvalue = ""
				otts.rnd = Randomness{}
			}
		case tValueFieldName:
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

// RValue returns the R-value (key: "rv") as a string or empty if
// there is no R-value set.
func (otts *OpenTelemetryTraceState) RValue() string {
	return otts.rvalue
}

// RValueRandomness returns the randomness object corresponding with
// RValue() and a boolean indicating whether the R-value is set.
func (otts *OpenTelemetryTraceState) RValueRandomness() (Randomness, bool) {
	return otts.rnd, len(otts.rvalue) != 0
}

// TValue returns the T-value (key: "th") as a string or empty if
// there is no T-value set.
func (otts *OpenTelemetryTraceState) TValue() string {
	return otts.tvalue
}

// TValueThreshold returns the threshold object corresponding with
// TValue() and a boolean (equal to len(TValue()) != 0 indicating
// whether the T-value is valid.
func (otts *OpenTelemetryTraceState) TValueThreshold() (Threshold, bool) {
	return otts.threshold, len(otts.tvalue) != 0
}

// UpdateTValueWithSampling modifies the TValue of this object, which
// changes its adjusted count.  If the change of TValue leads to
// inconsistency (i.e., raising sampling probability), an error is
// returned.
func (otts *OpenTelemetryTraceState) UpdateTValueWithSampling(sampledThreshold Threshold, encodedTValue string) error {
	if len(otts.TValue()) != 0 && ThresholdGreater(otts.threshold, sampledThreshold) {
		return ErrInconsistentSampling
	}
	otts.threshold = sampledThreshold
	otts.tvalue = encodedTValue
	return nil
}

// AdjustedCount returns the adjusted count implied by this TValue.
// This term is defined here:
// https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/
func (otts *OpenTelemetryTraceState) AdjustedCount() float64 {
	if len(otts.TValue()) == 0 {
		return 0
	}
	return 1.0 / otts.threshold.Probability()
}

// ClearTValue is used to unset TValue, in cases where it is
// inconsistent on arrival.
func (otts *OpenTelemetryTraceState) ClearTValue() {
	otts.tvalue = ""
	otts.threshold = Threshold{}
}

// SetRValue establishes explicit randomness for this TraceState.
func (otts *OpenTelemetryTraceState) SetRValue(randomness Randomness) {
	otts.rnd = randomness
	otts.rvalue = randomness.RValue()
}

// ClearRValue unsets explicit randomness.
func (otts *OpenTelemetryTraceState) ClearRValue() {
	otts.rvalue = ""
	otts.rnd = Randomness{}
}

// HasAnyValue returns true if there are any fields in this
// tracestate, including any extra values.
func (otts *OpenTelemetryTraceState) HasAnyValue() bool {
	return len(otts.RValue()) != 0 || len(otts.TValue()) != 0 || len(otts.ExtraValues()) != 0
}

// Serialize encodes this TraceState object.
func (otts *OpenTelemetryTraceState) Serialize(w io.StringWriter) error {
	ser := serializer{writer: w}
	cnt := 0
	sep := func() {
		if cnt != 0 {
			ser.write(";")
		}
		cnt++
	}
	if len(otts.RValue()) != 0 {
		sep()
		ser.write(rValueFieldName)
		ser.write(":")
		ser.write(otts.RValue())
	}
	if len(otts.TValue()) != 0 {
		sep()
		ser.write(tValueFieldName)
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
