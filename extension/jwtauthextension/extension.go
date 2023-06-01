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

package jwtauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jwtauthextension"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/auth"
	"go.uber.org/zap"
)

type jwtExtension struct {
	cfg       *Config
	jwtSecret []byte
	logger    *zap.Logger
}

var (
	errNoJWTSecretProvided               = errors.New("no jwt secret provided for the JWT configuration")
	errJWTInvalid                        = errors.New("the JWT is invalid")
	errInvalidAuthenticationHeaderFormat = errors.New("invalid authorization header format")
	errNotAuthenticated                  = errors.New("authentication didn't succeed")
)

func newExtension(cfg *Config, logger *zap.Logger) (auth.Server, error) {
	if cfg.JWTSecret == "" {
		return nil, errNoJWTSecretProvided
	}

	if cfg.Attribute == "" {
		cfg.Attribute = defaultAttribute
	}

	oe := &jwtExtension{
		cfg:       cfg,
		logger:    logger,
		jwtSecret: []byte(cfg.JWTSecret),
	}

	return auth.NewServer(auth.WithServerStart(oe.start), auth.WithServerAuthenticate(oe.authenticate)), nil
}

func (e *jwtExtension) start(context.Context, component.Host) error {
	return nil
}

// authenticate checks whether the given context contains valid auth data. Successfully authenticated calls will always return a nil error and a context with the auth data.
func (e *jwtExtension) authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	metadata := client.NewMetadata(headers)
	authHeaders := metadata.Get(e.cfg.Attribute)
	if len(authHeaders) == 0 {
		return ctx, errNotAuthenticated
	}

	// we only use the first header, if multiple values exist
	parts := strings.Split(authHeaders[0], " ")
	if len(parts) != 2 {
		return ctx, errInvalidAuthenticationHeaderFormat
	}

	rawToken := parts[1]

	token, err := jwt.ParseWithClaims(rawToken, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return e.jwtSecret, nil
	})
	if err != nil {
		return ctx, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errJWTInvalid
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		cl := client.FromContext(ctx)
		cl.Auth = &authData{
			jwtClaims: claims,
		}
		return client.NewContext(ctx, cl), nil
	}

	return ctx, fmt.Errorf("failed to get claims from token: %w", err)
}
