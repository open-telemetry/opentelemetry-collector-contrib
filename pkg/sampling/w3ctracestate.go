package sampling

import (
	"io"
	"regexp"
	"strconv"
)

type W3CTraceState struct {
	commonTraceState
	otts OTelTraceState
}

const (
	hardMaxW3CLength = 1024

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
	tenantIDRegexp       = lcDigitRegexp + lcDigitPunctRegexp + `{0,240}`
	systemIDRegexp       = lcAlphaRegexp + lcDigitPunctRegexp + `{0,13}`
	multiTenantKeyRegexp = tenantIDRegexp + `@` + systemIDRegexp
	simpleKeyRegexp      = lcAlphaRegexp + lcDigitPunctRegexp + `{0,255}`
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
	owsRegexp       = `[` + owsCharSet + `]*`
	w3cMemberRegexp = `(?:` + keyRegexp + `=` + valueRegexp + `)|(?:` + owsRegexp + `)`

	// This regexp is large enough that regexp impl refuses to
	// make 31 copies of it (i.e., `{0,31}`) so we use `*` below.
	w3cOwsCommaMemberRegexp = `(?:` + owsRegexp + `,` + owsRegexp + w3cMemberRegexp + `)`

	// The limit to 31 of owsCommaMemberRegexp is applied in code.
	w3cTracestateRegexp = `^` + w3cMemberRegexp + w3cOwsCommaMemberRegexp + `*$`
)

var (
	w3cTracestateRe = regexp.MustCompile(w3cTracestateRegexp)

	w3cSyntax = keyValueScanner{
		maxItems:  32,
		trim:      true,
		separator: ',',
		equality:  '=',
	}
)

func NewW3CTraceState(input string) (w3c W3CTraceState, _ error) {
	if len(input) > hardMaxW3CLength {
		return w3c, ErrTraceStateSize
	}

	if !w3cTracestateRe.MatchString(input) {
		return w3c, strconv.ErrSyntax
	}

	err := w3cSyntax.scanKeyValues(input, func(key, value string) error {
		switch key {
		case "ot":
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

func (w3c *W3CTraceState) HasAnyValue() bool {
	return w3c.HasOTelValue() || w3c.HasExtraValues()
}

func (w3c *W3CTraceState) OTelValue() *OTelTraceState {
	return &w3c.otts
}

func (w3c *W3CTraceState) HasOTelValue() bool {
	return w3c.otts.HasAnyValue()
}

func (w3c *W3CTraceState) Serialize(w io.StringWriter) {
	cnt := 0
	sep := func() {
		if cnt != 0 {
			w.WriteString(",")
		}
		cnt++
	}
	if w3c.otts.HasAnyValue() {
		sep()
		w.WriteString("ot=")
		w3c.otts.Serialize(w)
	}
	for _, kv := range w3c.ExtraValues() {
		sep()
		w.WriteString(kv.Key)
		w.WriteString("=")
		w.WriteString(kv.Value)
	}
}
