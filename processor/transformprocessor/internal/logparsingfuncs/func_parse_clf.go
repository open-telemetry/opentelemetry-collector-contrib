// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

// Supported values for the optional Format argument.
const (
	clfFormatCLF      = "clf"
	clfFormatCombined = "combined"
)

type parseCLFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
	Format ottl.Optional[string]
}

func NewParseCLFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("ParseCLF", &parseCLFArguments{}, createParseCLFFunction)
}

func createParseCLFFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseCLFArguments)
	if !ok {
		return nil, errors.New("parseCLFFactory args must be of type *parseCLFArguments")
	}

	format := clfFormatCLF
	if !args.Format.IsEmpty() {
		format = args.Format.Get()
	}
	switch format {
	case clfFormatCLF, clfFormatCombined:
	default:
		return nil, fmt.Errorf("invalid format %q: must be %q or %q", format, clfFormatCLF, clfFormatCombined)
	}

	return parseCLF(args.Target, format), nil
}

func parseCLF(target ottl.StringGetter[*ottllog.TransformContext], format string) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, errors.New("cannot parse empty CLF message")
		}

		return parseCLFMessage(source, format)
	}
}

// clfQuotedField matches the contents of a quoted CLF field, allowing
// backslash escapes (e.g. `\"`, `\\`, `\xhh`) as produced by Apache's
// mod_log_config. See
// https://httpd.apache.org/docs/current/mod/mod_log_config.html#format-notes
const clfQuotedField = `"((?:[^"\\]|\\.)*)"`

// clfRegex matches the Common Log Format:
//
//	remotehost rfc931 authuser [date] "request" status bytes
//
// See https://www.w3.org/Daemon/User/Config/Logging.html#common-logfile-format
var clfRegex = regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] ` + clfQuotedField + ` (\S+) (\S+)$`)

// combinedRegex matches the NCSA Combined Log Format, which is CLF with the
// quoted referer and user-agent appended:
//
//	remotehost rfc931 authuser [date] "request" status bytes "referer" "user-agent"
var combinedRegex = regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] ` + clfQuotedField + ` (\S+) (\S+) ` + clfQuotedField + ` ` + clfQuotedField + `$`)

// unescapeCLF reverses the escaping Apache's mod_log_config applies to quoted
// fields: `\"` and `\\` for the literal characters, C-style sequences such as
// `\n` and `\t` for control characters, and `\xhh` for other non-printable
// bytes (nginx uses the same `\xhh` form, e.g. `\x22` for a quote).
// Sequences that don't form a valid escape are preserved as-is.
func unescapeCLF(s string) string {
	if !strings.Contains(s, `\`) {
		// fast path — most fields have no escapes
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch next := s[i]; next {
		case '\\', '"':
			b.WriteByte(next)
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'x':
			if i+2 < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
					b.WriteByte(byte(v))
					i += 2
					continue
				}
			}
			b.WriteString(`\x`)
		default:
			// Not a recognized escape; keep the backslash and character.
			b.WriteByte('\\')
			b.WriteByte(next)
		}
	}
	return b.String()
}

func parseCLFMessage(message, format string) (pcommon.Map, error) {
	re := clfRegex
	if format == clfFormatCombined {
		re = combinedRegex
	}

	matches := re.FindStringSubmatch(strings.TrimSpace(message))
	if matches == nil {
		return pcommon.NewMap(), fmt.Errorf("invalid CLF message: does not match expected %q format", format)
	}

	result := pcommon.NewMap()
	result.PutStr("remote_host", matches[1])
	result.PutStr("rfc931", matches[2])
	result.PutStr("authuser", matches[3])
	result.PutStr("timestamp", matches[4])

	request := unescapeCLF(matches[5])
	result.PutStr("request", request)

	if requestParts := strings.SplitN(request, " ", 3); len(requestParts) == 3 {
		result.PutStr("method", requestParts[0])
		result.PutStr("request_uri", requestParts[1])
		result.PutStr("protocol", requestParts[2])
	}

	status := matches[6]
	statusInt, err := strconv.ParseInt(status, 10, 64)
	if err != nil {
		return pcommon.NewMap(), fmt.Errorf("invalid status code %q: %w", status, err)
	}
	result.PutInt("status", statusInt)

	bytesStr := matches[7]
	if bytesStr != "-" {
		bytesInt, err := strconv.ParseInt(bytesStr, 10, 64)
		if err != nil {
			return pcommon.NewMap(), fmt.Errorf("invalid bytes value %q: %w", bytesStr, err)
		}
		result.PutInt("bytes", bytesInt)
	}

	if format == clfFormatCombined {
		result.PutStr("referer", unescapeCLF(matches[8]))
		result.PutStr("user_agent", unescapeCLF(matches[9]))
	}

	return result, nil
}
