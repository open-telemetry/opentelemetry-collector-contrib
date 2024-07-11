// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/ua-parser/uap-go/uaparser"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
)

func userAgent[K any](userAgentSource ottl.StringGetter[K]) ottl.ExprFunc[K] { //revive:disable-line:var-naming
	return func(ctx context.Context, tCtx K) (any, error) {
		userAgentString, err := userAgentSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		// TODO: maybe we want to manage the regex definitions separately?
		parser := uaparser.NewFromSaved()
		parsedUserAgent := parser.ParseUserAgent(userAgentString)
		return map[string]any{
			semconv.AttributeUserAgentName:     parsedUserAgent.Family,
			semconv.AttributeUserAgentOriginal: userAgentString,
			semconv.AttributeUserAgentVersion:  parsedUserAgent.ToVersionString(),
		}, nil
	}
}
