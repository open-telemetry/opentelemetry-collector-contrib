// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/lumigoauthextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/auth"
	"go.uber.org/zap"
)

type LumigoAuthContextKey string

const (
	lumigoTokenContextKey LumigoAuthContextKey = "lumigo-token"
)

var (
	errNoAuth              = errors.New("the Authorization header is missing")
	errInvalidSchemePrefix = errors.New("the Authorization header does not have the 'LumigoToken <token>' structure")

	lumigoAuthValuePrefix = "LumigoToken "
)

type lumigoAuth struct {
	logger *zap.Logger
	cfg    *Config
}

func newClientAuthExtension(cfg *Config, logger *zap.Logger) (auth.Client, error) {
	if len(cfg.Token) > 0 {
		if err := ValidateLumigoToken(cfg.Token); err != nil {
			return nil, err
		}
	}

	la := lumigoAuth{
		logger: logger,
		cfg:    cfg,
	}

	return auth.NewClient(
		auth.WithClientRoundTripper(la.roundTripper),
	), nil
}

type lumigoAuthRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (l *lumigoAuthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	token := l.token
	if len(token) == 0 {
		ctxToken, ok := request.Context().Value(lumigoTokenContextKey).(string)

		if !ok {
			return nil, fmt.Errorf("no Lumigo token set in the configurations, and none found in the context")
		}

		token = ctxToken
	}

	newRequest := request.Clone(request.Context())
	newRequest.Header.Add("Authorization", lumigoAuthValuePrefix+token)

	return l.base.RoundTrip(newRequest)
}

func (la *lumigoAuth) roundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &lumigoAuthRoundTripper{
		base:  base,
		token: la.cfg.Token,
	}, nil
}

func newServerAuthExtension(cfg *Config, logger *zap.Logger) (auth.Server, error) {
	if logger == nil {
		return nil, fmt.Errorf("the provided logger is nil")
	}

	if len(cfg.Token) > 0 {
		return nil, fmt.Errorf("setting the 'token' field is not supported for LumigoAuth extensions used in receivers")
	}

	la := lumigoAuth{
		logger: logger,
		cfg:    cfg,
	}

	return auth.NewServer(
		auth.WithServerStart(la.serverStart),
		auth.WithServerAuthenticate(la.authenticate),
	), nil
}

func (la *lumigoAuth) serverStart(ctx context.Context, host component.Host) error {
	return nil
}

func (la *lumigoAuth) authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth := la.getAuthHeader(headers)
	if auth == "" {
		return ctx, errNoAuth
	}

	authData, err := la.parseLumigoToken(auth)
	if err != nil {
		return ctx, err
	}

	cl := client.FromContext(ctx)
	cl.Auth = authData
	return client.NewContext(ctx, cl), nil
}

func (la *lumigoAuth) getAuthHeader(h map[string][]string) string {
	const (
		canonicalHeaderKey = "Authorization"
		metadataKey        = "authorization"
	)

	authHeaders, ok := h[canonicalHeaderKey]

	if !ok {
		authHeaders, ok = h[metadataKey]
	}

	if !ok {
		for k, v := range h {
			if strings.EqualFold(k, metadataKey) {
				authHeaders = v
				break
			}
		}
	}

	if len(authHeaders) == 0 {
		return ""
	}

	return authHeaders[0]
}

func (la *lumigoAuth) parseLumigoToken(auth string) (*authData, error) {
	if len(auth) < len(lumigoAuthValuePrefix) || !strings.EqualFold(auth[:len(lumigoAuthValuePrefix)], lumigoAuthValuePrefix) {
		return nil, errInvalidSchemePrefix
	}

	token := auth[len(lumigoAuthValuePrefix):]

	return &authData{
		raw:   auth,
		token: token,
	}, nil
}

var _ client.AuthData = (*authData)(nil)

type authData struct {
	raw   string
	token string
}

func (a *authData) GetAttribute(name string) interface{} {
	switch name {
	case string(lumigoTokenContextKey):
		return a.token
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"raw", "lumigo-token"}
}
