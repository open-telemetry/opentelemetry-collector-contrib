// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
)

const (
	// replace once conventions includes these
	AttributeURLUserInfo = "url.user_info"
	AttributeURLUsername = "url.username"
	AttributeURLPassword = "url.password"
)

type URIArguments[K any] struct {
	URI ottl.StringGetter[K]
}

func NewURIFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("URI", &URIArguments[K]{}, createURIFunction[K])
}

func createURIFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*URIArguments[K])
	if !ok {
		return nil, fmt.Errorf("URIFactory args must be of type *URIArguments[K]")
	}

	return uri(args.URI), nil //revive:disable-line:var-naming
}

func uri[K any](uriSource ottl.StringGetter[K]) ottl.ExprFunc[K] { //revive:disable-line:var-naming
	return func(ctx context.Context, tCtx K) (any, error) {
		uriString, err := uriSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if uriString == "" {
			return nil, fmt.Errorf("uri cannot be nil")
		}

		uriParts := make(map[string]any)

		parsedURI, err := url.Parse(uriString)
		if err != nil {
			return nil, err
		}

		// always present fields
		uriParts[semconv.AttributeURLOriginal] = uriString
		uriParts[semconv.AttributeURLDomain] = parsedURI.Hostname()
		uriParts[semconv.AttributeURLScheme] = parsedURI.Scheme
		uriParts[semconv.AttributeURLPath] = parsedURI.Path

		// optional fields included only if populated
		if port := parsedURI.Port(); len(port) > 0 {
			uriParts[semconv.AttributeURLPort], err = strconv.Atoi(port)
			if err != nil {
				return nil, err
			}
		}

		if fragment := parsedURI.Fragment; len(fragment) > 0 {
			uriParts[semconv.AttributeURLFragment] = fragment
		}

		if parsedURI.User != nil {
			uriParts[AttributeURLUserInfo] = parsedURI.User.String()

			if username := parsedURI.User.Username(); len(username) > 0 {
				uriParts[AttributeURLUsername] = username
			}

			if pwd, isSet := parsedURI.User.Password(); isSet {
				uriParts[AttributeURLPassword] = pwd
			}
		}

		if query := parsedURI.RawQuery; len(query) > 0 {
			uriParts[semconv.AttributeURLQuery] = query
		}

		if periodIdx := strings.LastIndex(parsedURI.Path, "."); periodIdx != -1 {
			if periodIdx < len(parsedURI.Path)-1 {
				uriParts[semconv.AttributeURLExtension] = parsedURI.Path[periodIdx+1:]
			}
		}

		return uriParts, nil
	}
}
