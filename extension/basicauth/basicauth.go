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
	"errors"
	"fmt"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
)

var _ configauth.ServerAuthenticator = (*BasicAuth)(nil)

type BasicAuth struct {
	htpasswd  string
	matchFunc matchFunc
}

type matchFunc func(username, password string) bool

func (ba *BasicAuth) Start(ctx context.Context, host component.Host) error {
	htp, err := htpasswd.New(ba.htpasswd, htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("new htpasswd: %w", err)
	}
	ba.matchFunc = htp.Match
	return nil
}

func (ba *BasicAuth) Shutdown(ctx context.Context) error {
	return nil
}

var (
	errNoAuth             = errors.New("no basic auth provided")
	errInvalidCredentials = errors.New("invalid credentials")
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
