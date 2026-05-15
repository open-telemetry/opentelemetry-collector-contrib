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

type parseCLFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

func NewParseCLFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("parse_clf", &parseCLFArguments{}, createParseCLFFunction)
}

func createParseCLFFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseCLFArguments)
	if !ok {
		return nil, errors.New("parseCLFFactory args must be of type *parseCLFArguments")
	}

	return parseCLF(args.Target), nil
}

func parseCLF(target ottl.StringGetter[*ottllog.TransformContext]) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, errors.New("cannot parse empty CLF message")
		}

		return parseCLFMessage(source)
	}
}

// clfRegex matches the Common Log Format:
//
//	remotehost rfc931 authuser [date] "request" status bytes
//
// See https://www.w3.org/Daemon/User/Config/Logging.html#common-logfile-format
var clfRegex = regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] "([^"]*)" (\S+) (\S+)$`)

func parseCLFMessage(message string) (pcommon.Map, error) {
	matches := clfRegex.FindStringSubmatch(strings.TrimSpace(message))
	if matches == nil {
		return pcommon.NewMap(), errors.New("invalid CLF message: does not match expected format")
	}

	result := pcommon.NewMap()
	result.PutStr("remote_host", matches[1])
	result.PutStr("rfc931", matches[2])
	result.PutStr("authuser", matches[3])
	result.PutStr("timestamp", matches[4])

	request := matches[5]
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

	return result, nil
}
