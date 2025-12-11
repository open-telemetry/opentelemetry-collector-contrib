// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/otel/semconv/v1.37.0"

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
				string(conventions.URLPathKey):      "/foo.gif",
				string(conventions.URLFragmentKey):  "fragment",
				string(conventions.URLExtensionKey): "gif",
				AttributeURLPassword:                "mypassword",
				string(conventions.URLOriginalKey):  "http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
				string(conventions.URLSchemeKey):    "http",
				string(conventions.URLPortKey):      80,
				AttributeURLUserInfo:                "myusername:mypassword",
				string(conventions.URLDomainKey):    "www.example.com",
				string(conventions.URLQueryKey):     "key1=val1&key2=val2",
				AttributeURLUsername:                "myusername",
			},
		},
		{
			"simple example",
			"http://www.example.com",
			map[string]any{
				string(conventions.URLOriginalKey): "http://www.example.com",
				string(conventions.URLSchemeKey):   "http",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "",
			},
		},
		{
			"custom port",
			"http://www.example.com:77",
			map[string]any{
				string(conventions.URLOriginalKey): "http://www.example.com:77",
				string(conventions.URLSchemeKey):   "http",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "",
				string(conventions.URLPortKey):     77,
			},
		},
		{
			"file",
			"http://www.example.com:77/file.png",
			map[string]any{
				string(conventions.URLOriginalKey):  "http://www.example.com:77/file.png",
				string(conventions.URLSchemeKey):    "http",
				string(conventions.URLDomainKey):    "www.example.com",
				string(conventions.URLPathKey):      "/file.png",
				string(conventions.URLPortKey):      77,
				string(conventions.URLExtensionKey): "png",
			},
		},
		{
			"fragment",
			"http://www.example.com:77/foo#bar",
			map[string]any{
				string(conventions.URLOriginalKey): "http://www.example.com:77/foo#bar",
				string(conventions.URLSchemeKey):   "http",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "/foo",
				string(conventions.URLPortKey):     77,
				string(conventions.URLFragmentKey): "bar",
			},
		},
		{
			"query example",
			"https://www.example.com:77/foo?key=val",
			map[string]any{
				string(conventions.URLOriginalKey): "https://www.example.com:77/foo?key=val",
				string(conventions.URLSchemeKey):   "https",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "/foo",
				string(conventions.URLPortKey):     77,
				string(conventions.URLQueryKey):    "key=val",
			},
		},
		{
			"user info",
			"https://user:pw@www.example.com:77/foo",
			map[string]any{
				string(conventions.URLOriginalKey): "https://user:pw@www.example.com:77/foo",
				string(conventions.URLSchemeKey):   "https",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "/foo",
				string(conventions.URLPortKey):     77,
				AttributeURLUserInfo:               "user:pw",
				AttributeURLUsername:               "user",
				AttributeURLPassword:               "pw",
			},
		},
		{
			"user info - no password",
			"https://user:@www.example.com:77/foo",
			map[string]any{
				string(conventions.URLOriginalKey): "https://user:@www.example.com:77/foo",
				string(conventions.URLSchemeKey):   "https",
				string(conventions.URLDomainKey):   "www.example.com",
				string(conventions.URLPathKey):     "/foo",
				string(conventions.URLPortKey):     77,
				AttributeURLUserInfo:               "user:",
				AttributeURLUsername:               "user",
				AttributeURLPassword:               "",
			},
		},
		{
			"non-http scheme: ftp",
			"ftp://ftp.is.co.za/rfc/rfc1808.txt",
			map[string]any{
				string(conventions.URLOriginalKey):  "ftp://ftp.is.co.za/rfc/rfc1808.txt",
				string(conventions.URLSchemeKey):    "ftp",
				string(conventions.URLPathKey):      "/rfc/rfc1808.txt",
				string(conventions.URLExtensionKey): "txt",
				string(conventions.URLDomainKey):    "ftp.is.co.za",
			},
		},
		{
			"non-http scheme: telnet",
			"telnet://192.0.2.16:80/",
			map[string]any{
				string(conventions.URLOriginalKey): "telnet://192.0.2.16:80/",
				string(conventions.URLSchemeKey):   "telnet",
				string(conventions.URLPathKey):     "/",
				string(conventions.URLPortKey):     80,
				string(conventions.URLDomainKey):   "192.0.2.16",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			source := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tc.Original, nil
				},
			}

			exprFunc := url(source) //revive:disable-line:var-naming
			res, err := exprFunc(t.Context(), nil)
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
