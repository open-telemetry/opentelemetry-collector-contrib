// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	// replace once conventions includes these
	AttributeURLUserInfo = "url.user_info"
	AttributeURLUsername = "url.username"
	AttributeURLPassword = "url.password"
)

func TestURLParser(t *testing.T) {
	testCases := []struct {
		Name        string
		Original    string
		ExpectedMap map[string]any
	}{
		{
			"complete example",
			"http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
			map[string]any{
				string(semconv.URLPathKey):      "/foo.gif",
				string(semconv.URLFragmentKey):  "fragment",
				string(semconv.URLExtensionKey): "gif",
				AttributeURLPassword:            "mypassword",
				string(semconv.URLOriginalKey):  "http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
				string(semconv.URLSchemeKey):    "http",
				string(semconv.URLPortKey):      80,
				AttributeURLUserInfo:            "myusername:mypassword",
				string(semconv.URLDomainKey):    "www.example.com",
				string(semconv.URLQueryKey):     "key1=val1&key2=val2",
				AttributeURLUsername:            "myusername",
			},
		},
		{
			"simple example",
			"http://www.example.com",
			map[string]any{
				string(semconv.URLOriginalKey): "http://www.example.com",
				string(semconv.URLSchemeKey):   "http",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "",
			},
		},
		{
			"custom port",
			"http://www.example.com:77",
			map[string]any{
				string(semconv.URLOriginalKey): "http://www.example.com:77",
				string(semconv.URLSchemeKey):   "http",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "",
				string(semconv.URLPortKey):     77,
			},
		},
		{
			"file",
			"http://www.example.com:77/file.png",
			map[string]any{
				string(semconv.URLOriginalKey):  "http://www.example.com:77/file.png",
				string(semconv.URLSchemeKey):    "http",
				string(semconv.URLDomainKey):    "www.example.com",
				string(semconv.URLPathKey):      "/file.png",
				string(semconv.URLPortKey):      77,
				string(semconv.URLExtensionKey): "png",
			},
		},
		{
			"fragment",
			"http://www.example.com:77/foo#bar",
			map[string]any{
				string(semconv.URLOriginalKey): "http://www.example.com:77/foo#bar",
				string(semconv.URLSchemeKey):   "http",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "/foo",
				string(semconv.URLPortKey):     77,
				string(semconv.URLFragmentKey): "bar",
			},
		},
		{
			"query example",
			"https://www.example.com:77/foo?key=val",
			map[string]any{
				string(semconv.URLOriginalKey): "https://www.example.com:77/foo?key=val",
				string(semconv.URLSchemeKey):   "https",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "/foo",
				string(semconv.URLPortKey):     77,
				string(semconv.URLQueryKey):    "key=val",
			},
		},
		{
			"user info",
			"https://user:pw@www.example.com:77/foo",
			map[string]any{
				string(semconv.URLOriginalKey): "https://user:pw@www.example.com:77/foo",
				string(semconv.URLSchemeKey):   "https",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "/foo",
				string(semconv.URLPortKey):     77,
				AttributeURLUserInfo:           "user:pw",
				AttributeURLUsername:           "user",
				AttributeURLPassword:           "pw",
			},
		},
		{
			"user info - no password",
			"https://user:@www.example.com:77/foo",
			map[string]any{
				string(semconv.URLOriginalKey): "https://user:@www.example.com:77/foo",
				string(semconv.URLSchemeKey):   "https",
				string(semconv.URLDomainKey):   "www.example.com",
				string(semconv.URLPathKey):     "/foo",
				string(semconv.URLPortKey):     77,
				AttributeURLUserInfo:           "user:",
				AttributeURLUsername:           "user",
				AttributeURLPassword:           "",
			},
		},
		{
			"non-http scheme: ftp",
			"ftp://ftp.is.co.za/rfc/rfc1808.txt",
			map[string]any{
				string(semconv.URLOriginalKey):  "ftp://ftp.is.co.za/rfc/rfc1808.txt",
				string(semconv.URLSchemeKey):    "ftp",
				string(semconv.URLPathKey):      "/rfc/rfc1808.txt",
				string(semconv.URLExtensionKey): "txt",
				string(semconv.URLDomainKey):    "ftp.is.co.za",
			},
		},
		{
			"non-http scheme: telnet",
			"telnet://192.0.2.16:80/",
			map[string]any{
				string(semconv.URLOriginalKey): "telnet://192.0.2.16:80/",
				string(semconv.URLSchemeKey):   "telnet",
				string(semconv.URLPathKey):     "/",
				string(semconv.URLPortKey):     80,
				string(semconv.URLDomainKey):   "192.0.2.16",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			source := &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tc.Original, nil
				},
			}

			exprFunc := url(source) //revive:disable-line:var-naming
			res, err := exprFunc(context.Background(), nil)
			require.NoError(t, err)

			resMap, ok := res.(map[string]any)
			require.True(t, ok)

			require.Len(t, resMap, len(tc.ExpectedMap))
			for k, v := range tc.ExpectedMap {
				actualValue, found := resMap[k]
				require.True(t, found, "key not found %q", k)
				require.Equal(t, v, actualValue)
			}
		})
	}
}
