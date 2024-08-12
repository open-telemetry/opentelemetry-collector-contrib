// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/ua-parser/uap-go/uaparser"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
)

// userAgentParser holds the reference to a lazily initialized user agent parser
var userAgentParser *uaparser.Parser

// uaParserInitOnce is used to lazily initialize the userAgentParser variable
var uaParserInitOnce sync.Once

type UserAgentArguments[K any] struct {
	UserAgent ottl.StringGetter[K]
}

func NewUserAgentFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("UserAgent", &UserAgentArguments[K]{}, createUserAgentFunction[K])
}

func createUserAgentFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*UserAgentArguments[K])
	if !ok {
		return nil, fmt.Errorf("URLFactory args must be of type *URLArguments[K]")
	}

	return userAgent[K](args.UserAgent), nil
}

func uaParser() *uaparser.Parser {
	uaParserInitOnce.Do(func() {
		userAgentParser = uaparser.NewFromSaved()
	})
	return userAgentParser
}

func userAgent[K any](userAgentSource ottl.StringGetter[K]) ottl.ExprFunc[K] { //revive:disable-line:var-naming
	return func(ctx context.Context, tCtx K) (any, error) {
		userAgentString, err := userAgentSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		parsedUserAgent := uaParser().ParseUserAgent(userAgentString)
		return map[string]any{
			semconv.AttributeUserAgentName:     parsedUserAgent.Family,
			semconv.AttributeUserAgentOriginal: userAgentString,
			semconv.AttributeUserAgentVersion:  parsedUserAgent.ToVersionString(),
		}, nil
	}
}
