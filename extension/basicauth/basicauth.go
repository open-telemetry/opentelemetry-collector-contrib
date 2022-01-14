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

package basicauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauth"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
)

var _ configauth.ServerAuthenticator = (*BasicAuth)(nil)

type BasicAuth struct {
	htpasswd  HtpasswdSettings
	matchFunc matchFunc
}

type matchFunc func(username, password string) bool

func (ba *BasicAuth) Start(ctx context.Context, host component.Host) error {
	inlineHtp, err := htpasswd.NewFromReader(strings.NewReader(ba.htpasswd.Inline), htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("read htpasswd from inline: %w", err)
	}
	ba.matchFunc = inlineHtp.Match

	if ba.htpasswd.File != "" {
		fileHtp, err := htpasswd.New(ba.htpasswd.File, htpasswd.DefaultSystems, nil)
		if err != nil {
			return fmt.Errorf("read htpasswd from file: %w", err)
		}

		matchFunc := ba.matchFunc
		ba.matchFunc = func(username, password string) bool {
			// Inline takes precedence over the file.
			return matchFunc(username, password) || fileHtp.Match(username, password)
		}
	}

	return nil
}

func (ba *BasicAuth) Shutdown(ctx context.Context) error {
	return nil
}

var (
	errNoAuth              = errors.New("no basic auth provided")
	errInvalidCredentials  = errors.New("invalid credentials")
	errInvalidSchemePrefix = errors.New("invalid authorization scheme prefix")
	errInvalidFormat       = errors.New("invalid authorization format")
)

func (ba *BasicAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	authHeaders := headers["authorization"]
	if len(authHeaders) == 0 {
		return ctx, errNoAuth
	}

	authData, err := parseBasicAuth(authHeaders[0])
	if err != nil {
		return ctx, err
	}

	if !ba.matchFunc(authData.username, authData.password) {
		return ctx, errInvalidCredentials
	}

	cl := client.FromContext(ctx)
	cl.Auth = authData
	return client.NewContext(ctx, cl), nil
}

// See: https://github.com/golang/go/blob/1a8b4e05b1ff7a52c6d40fad73bcad612168d094/src/net/http/request.go#L950
func parseBasicAuth(auth string) (*authData, error) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil, errInvalidSchemePrefix
	}

	encoded := auth[len(prefix):]
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidFormat
	}
	decoded := string(decodedBytes)

	si := strings.IndexByte(decoded, ':')
	if si < 0 {
		return nil, errInvalidFormat
	}

	return &authData{
		username: decoded[:si],
		password: decoded[si+1:],
		raw:      encoded,
	}, nil
}

var _ client.AuthData = (*authData)(nil)

type authData struct {
	username string
	password string
	raw      string
}

func (a *authData) GetAttribute(name string) interface{} {
	switch name {
	case "subject":
		return a.username
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"subject", "raw"}
}
