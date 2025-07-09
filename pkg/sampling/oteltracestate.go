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
				// RValueRandomness() will return false, the error
				// accumulates and is returned below.
				otts.rvalue = ""
				otts.rnd = Randomness{}
			}
		case tValueFieldName:
			if otts.threshold, err = TValueToThreshold(value); err == nil {
				otts.tvalue = value
			} else {
				// TValueThreshold() will return false, the error
				// accumulates and is returned below.
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
// changes its adjusted count.  It is not logical to modify a sampling
// probability in the direction of larger probability.  This prevents
// accidental loss of adjusted count.
//
// If the change of TValue leads to inconsistency, an error is returned.
func (otts *OpenTelemetryTraceState) UpdateTValueWithSampling(sampledThreshold Threshold) error {
	// Note: there was once a code path here that optimized for
	// cases where a static threshold is used, in which case the
	// call to TValue() causes an unnecessary allocation per data
	// item (w/ a constant result).  We have eliminated that
	// parameter, due to the significant potential for mis-use.
	// Therefore, this method always recomputes TValue() of the
	// sampledThreshold (on success).  A future method such as
	// UpdateTValueWithSamplingFixedTValue() could extend this
	// API to address this allocation, although it is probably
	// not significant.
	if len(otts.TValue()) != 0 && ThresholdGreater(otts.threshold, sampledThreshold) {
		return ErrInconsistentSampling
	}
	// Note NeverSampleThreshold is the (exclusive) upper boundary
	// of valid thresholds, so the test above permits never-
	// sampled updates, in which case the TValue() here is empty.
	otts.threshold = sampledThreshold
	otts.tvalue = sampledThreshold.TValue()
	return nil
}

// AdjustedCount returns the adjusted count for this item.  If the
// TValue string is empty, this returns 0, otherwise returns
// Threshold.AdjustedCount().
func (otts *OpenTelemetryTraceState) AdjustedCount() float64 {
	if len(otts.tvalue) == 0 {
		// Note: this case covers the zero state, where
		// len(tvalue) == 0 and threshold == AlwaysSampleThreshold.
		// We return 0 to indicate that no information is available.
		return 0
	}
	return otts.threshold.AdjustedCount()
}

// ClearTValue is used to unset TValue, for use in cases where it is
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
