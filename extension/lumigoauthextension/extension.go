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
	"strings"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
)

var (
	errNoAuth              = errors.New("the Authorization header is missing")
	errInvalidSchemePrefix = errors.New("the Authorization header does not have the 'LumigoToken <token>' structure")
)

type lumigoAuth struct {
	logger *zap.Logger
}

func newServerAuthExtension(cfg *Config, logger *zap.Logger) (configauth.ServerAuthenticator, error) {
	la := lumigoAuth{
		logger: logger,
	}
	return configauth.NewServerAuthenticator(
		configauth.WithStart(la.serverStart),
		configauth.WithAuthenticate(la.authenticate),
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
	const prefix = "LumigoToken "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil, errInvalidSchemePrefix
	}

	token := auth[len(prefix):]

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
	case "lumigo-token":
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
