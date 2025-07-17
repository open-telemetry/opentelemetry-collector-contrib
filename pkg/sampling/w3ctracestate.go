// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"io"
	"regexp"
	"strconv"
	"strings"
)

// W3CTraceState represents the a parsed W3C `tracestate` header.
//
// This type receives and passes through `tracestate` fields defined
// by all vendors, while it parses and validates the
// [OpenTelemetryTraceState] field.  After parsing the W3CTraceState,
// access the OpenTelemetry-defined fields using
// [W3CTraceState.OTelValue].
type W3CTraceState struct {
	// commonTraceState holds "extra" values (e.g.,
	// vendor-specific tracestate fields) which are propagated but
	// not used by Sampling logic.
	commonTraceState

	// otts stores OpenTelemetry-specified tracestate fields.
	otts OpenTelemetryTraceState
}

const (
	hardMaxNumPairs     = 32
	hardMaxW3CLength    = 1024
	hardMaxKeyLength    = 256
	hardMaxTenantLength = 241
	hardMaxSystemLength = 14

	otelVendorCode = "ot"

	// keyRegexp is not an exact test, it permits all the
	// characters and then we check various conditions.

	// key              = simple-key / multi-tenant-key
	// simple-key       = lcalpha 0*255( lcalpha / DIGIT / "_" / "-"/ "*" / "/" )
	// multi-tenant-key = tenant-id "@" system-id
	// tenant-id        = ( lcalpha / DIGIT ) 0*240( lcalpha / DIGIT / "_" / "-"/ "*" / "/" )
	// system-id        = lcalpha 0*13( lcalpha / DIGIT / "_" / "-"/ "*" / "/" )
	// lcalpha          = %x61-7A ; a-z

	lcAlphaRegexp         = `[a-z]`
	lcAlphanumPunctRegexp = `[a-z0-9\-\*/_]`
	lcAlphanumRegexp      = `[a-z0-9]`
	multiTenantSep        = `@`
	tenantIDRegexp        = lcAlphanumRegexp + lcAlphanumPunctRegexp + `*`
	systemIDRegexp        = lcAlphaRegexp + lcAlphanumPunctRegexp + `*`
	multiTenantKeyRegexp  = tenantIDRegexp + multiTenantSep + systemIDRegexp
	simpleKeyRegexp       = lcAlphaRegexp + lcAlphanumPunctRegexp + `*`
	keyRegexp             = `(?:(?:` + simpleKeyRegexp + `)|(?:` + multiTenantKeyRegexp + `))`

	// value    = 0*255(chr) nblk-chr
	// nblk-chr = %x21-2B / %x2D-3C / %x3E-7E
	// chr      = %x20 / nblk-chr
	//
	// Note the use of double-quoted strings in two places below.
	// This is for \x expansion in these two cases.  Also note
	// \x2d is a hyphen character, so a quoted \ (i.e., \\\x2d)
	// appears below.
	valueNonblankCharRegexp = "[\x21-\x2b\\\x2d-\x3c\x3e-\x7e]"
	valueCharRegexp         = "[\x20-\x2b\\\x2d-\x3c\x3e-\x7e]"
	valueRegexp             = valueCharRegexp + `{0,255}` + valueNonblankCharRegexp

	// tracestate  = list-member 0*31( OWS "," OWS list-member )
	// list-member = (key "=" value) / OWS

	owsCharSet      = ` \t`
	owsRegexp       = `(?:[` + owsCharSet + `]*)`
	w3cMemberRegexp = `(?:` + keyRegexp + `=` + valueRegexp + `)?`

	w3cOwsMemberOwsRegexp      = `(?:` + owsRegexp + w3cMemberRegexp + owsRegexp + `)`
	w3cCommaOwsMemberOwsRegexp = `(?:` + `,` + w3cOwsMemberOwsRegexp + `)`

	w3cTracestateRegexp = `^` + w3cOwsMemberOwsRegexp + w3cCommaOwsMemberOwsRegexp + `*$`

	// Note that fixed limits on tracestate size are captured above
	// as '*' regular expressions, which allows the parser to exceed
	// fixed limits, which are checked in code.  This keeps the size
	// of the compiled regexp reasonable.  Some of the regexps above
	// are too complex to expand e.g., 31 times.  In the case of
	// w3cTracestateRegexp, 32 elements are allowed, which means we
	// want the w3cCommaOwsMemberOwsRegexp element to match at most
	// 31 times, but this is checked in code.
)

var (
	w3cTracestateRe = regexp.MustCompile(w3cTracestateRegexp)

	w3cSyntax = keyValueScanner{
		maxItems:  hardMaxNumPairs,
		trim:      true,
		separator: ',',
		equality:  '=',
	}
)

// NewW3CTraceState parses a W3C trace state, with special attention
// to the embedded OpenTelemetry trace state field.
func NewW3CTraceState(input string) (w3c W3CTraceState, _ error) {
	if len(input) > hardMaxW3CLength {
		return w3c, ErrTraceStateSize
	}

	if !w3cTracestateRe.MatchString(input) {
		return w3c, strconv.ErrSyntax
	}

	err := w3cSyntax.scanKeyValues(input, func(key, value string) error {
		if len(key) > hardMaxKeyLength {
			return ErrTraceStateSize
		}
		if tenant, system, found := strings.Cut(key, multiTenantSep); found {
			if len(tenant) > hardMaxTenantLength {
				return ErrTraceStateSize
			}
			if len(system) > hardMaxSystemLength {
				return ErrTraceStateSize
			}
		}
		switch key {
		case otelVendorCode:
			var err error
			w3c.otts, err = NewOpenTelemetryTraceState(value)
			return err
		default:
			w3c.kvs = append(w3c.kvs, KV{
				Key:   key,
				Value: value,
			})
			return nil
		}
	})
	return w3c, err
}

// HasAnyValue indicates whether there are any values in this
// tracestate, including extra values.
func (w3c *W3CTraceState) HasAnyValue() bool {
	return w3c.OTelValue().HasAnyValue() || len(w3c.ExtraValues()) != 0
}

// OTelValue returns the OpenTelemetry tracestate value.
func (w3c *W3CTraceState) OTelValue() *OpenTelemetryTraceState {
	return &w3c.otts
}

// Serialize encodes this tracestate object for use as a W3C
// tracestate header value.
func (w3c *W3CTraceState) Serialize(w io.StringWriter) error {
	ser := serializer{writer: w}
	cnt := 0
	sep := func() {
		if cnt != 0 {
			ser.write(",")
		}
		cnt++
	}
	if w3c.otts.HasAnyValue() {
		sep()
		ser.write("ot=")
		ser.check(w3c.otts.Serialize(w))
	}
	for _, kv := range w3c.ExtraValues() {
		sep()
		ser.write(kv.Key)
		ser.write("=")
		ser.write(kv.Value)
	}
	return ser.err
}
