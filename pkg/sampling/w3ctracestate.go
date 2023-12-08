// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"io"
	"regexp"
	"strconv"
	"strings"
)

type W3CTraceState struct {
	commonTraceState
	otts OTelTraceState
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

	lcAlphaRegexp        = `[a-z]`
	lcDigitPunctRegexp   = `[a-z0-9\-\*/_]`
	lcDigitRegexp        = `[a-z0-9]`
	multiTenantSep       = `@`
	tenantIDRegexp       = lcDigitRegexp + lcDigitPunctRegexp + `*` // could be {0,hardMaxTenantLength-1}
	systemIDRegexp       = lcAlphaRegexp + lcDigitPunctRegexp + `*` // could be {0,hardMaxSystemLength-1}
	multiTenantKeyRegexp = tenantIDRegexp + multiTenantSep + systemIDRegexp
	simpleKeyRegexp      = lcAlphaRegexp + lcDigitPunctRegexp + `*` // could be {0,hardMaxKeyLength-1}
	keyRegexp            = `(?:(?:` + simpleKeyRegexp + `)|(?:` + multiTenantKeyRegexp + `))`

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

	// This regexp is large enough that regexp impl refuses to
	// make 31 copies of it (i.e., `{0,31}`) so we use `*` below.
	w3cOwsMemberOwsRegexp      = `(?:` + owsRegexp + w3cMemberRegexp + owsRegexp + `)`
	w3cCommaOwsMemberOwsRegexp = `(?:` + `,` + w3cOwsMemberOwsRegexp + `)`

	// The limit to 31 of owsCommaMemberRegexp is applied in code.
	w3cTracestateRegexp = `^` + w3cOwsMemberOwsRegexp + w3cCommaOwsMemberOwsRegexp + `*$`
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
			w3c.otts, err = NewOTelTraceState(value)
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
	return w3c.HasOTelValue() || w3c.HasExtraValues()
}

// OTelValue returns the OpenTelemetry tracestate value.
func (w3c *W3CTraceState) OTelValue() *OTelTraceState {
	return &w3c.otts
}

// HasOTelValue indicates whether an OpenTelemetry tracestate value
// is present in this tracestate.
func (w3c *W3CTraceState) HasOTelValue() bool {
	return w3c.otts.HasAnyValue()
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
