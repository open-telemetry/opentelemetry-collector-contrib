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

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	creds "google.golang.org/grpc/credentials"
)

var (
	errNoCredentialSource  = errors.New("no credential source provided")
	errNoAuth              = errors.New("no basic auth provided")
	errInvalidCredentials  = errors.New("invalid credentials")
	errInvalidSchemePrefix = errors.New("invalid authorization scheme prefix")
	errInvalidFormat       = errors.New("invalid authorization format")
)

type basicAuth struct {
	htpasswd     HtpasswdSettings
	matchFunc    func(username, password string) bool
	userPassPair string
}

func newExtension(cfg *Config) (*basicAuth, error) {
	if cfg.Htpasswd.File == "" && cfg.Htpasswd.Inline == "" {
		return nil, errNoCredentialSource
	}
	ba := basicAuth{
		htpasswd: cfg.Htpasswd,
	}
	return &ba, nil
}

func (ba *basicAuth) Start(ctx context.Context, host component.Host) error {
	var buff bytes.Buffer

	if ba.htpasswd.File != "" {
		bytes, err := ioutil.ReadFile(ba.htpasswd.File)
		if err != nil {
			return fmt.Errorf("open file error: %w", err)
		}
		buff.Write(bytes)
		buff.WriteString("\n")
	}

	// Ensure that the inline content is read the last.
	// This way the inline content will override the content from file.
	if len(ba.htpasswd.Inline) > 0 {
		buff.Truncate(buff.Len())
		buff.WriteString(ba.htpasswd.Inline)
	}

	htp, err := htpasswd.NewFromReader(bytes.NewBuffer(buff.Bytes()), htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("read htpasswd content: %w", err)
	}
	ba.userPassPair = buff.String()
	ba.matchFunc = htp.Match
	return nil
}

func (ba *basicAuth) Shutdown(ctx context.Context) error {
	return nil
}

func (ba *basicAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth := getAuthHeader(headers)
	if auth == "" {
		return ctx, errNoAuth
	}

	authData, err := parseBasicAuth(auth)
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

func getAuthHeader(h map[string][]string) string {
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

func getBasicAuth(auth string) (*authData, error) {
	si := strings.Index(auth, ":")
	if si < 0 {
		return nil, errInvalidFormat
	}
	return &authData{
		username: auth[:si],
		password: auth[si+1:],
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
	case "username":
		return a.username
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"username", "raw"}
}

type basicAuthRoundTripper struct {
	base     http.RoundTripper
	authData *authData
}

func (b *basicAuthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	newRequest := request.Clone(request.Context())
	newRequest.SetBasicAuth(b.authData.username, b.authData.password)
	return b.base.RoundTrip(newRequest)
}

func (ba *basicAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	authData, err := getBasicAuth(ba.userPassPair)
	if err != nil {
		return nil, err
	}
	return &basicAuthRoundTripper{
		base:     base,
		authData: authData,
	}, nil
}

func (ba *basicAuth) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	return nil, nil
}
