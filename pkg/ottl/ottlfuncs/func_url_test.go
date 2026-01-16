// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

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
				"url.path":           "/foo.gif",
				"url.fragment":       "fragment",
				"url.extension":      "gif",
				AttributeURLPassword: "mypassword",
				"url.original":       "http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
				"url.scheme":         "http",
				"url.port":           80,
				AttributeURLUserInfo: "myusername:mypassword",
				"url.domain":         "www.example.com",
				"url.query":          "key1=val1&key2=val2",
				AttributeURLUsername: "myusername",
			},
		},
		{
			"simple example",
			"http://www.example.com",
			map[string]any{
				"url.original": "http://www.example.com",
				"url.scheme":   "http",
				"url.domain":   "www.example.com",
				"url.path":     "",
			},
		},
		{
			"custom port",
			"http://www.example.com:77",
			map[string]any{
				"url.original": "http://www.example.com:77",
				"url.scheme":   "http",
				"url.domain":   "www.example.com",
				"url.path":     "",
				"url.port":     77,
			},
		},
		{
			"file",
			"http://www.example.com:77/file.png",
			map[string]any{
				"url.original":  "http://www.example.com:77/file.png",
				"url.scheme":    "http",
				"url.domain":    "www.example.com",
				"url.path":      "/file.png",
				"url.port":      77,
				"url.extension": "png",
			},
		},
		{
			"fragment",
			"http://www.example.com:77/foo#bar",
			map[string]any{
				"url.original": "http://www.example.com:77/foo#bar",
				"url.scheme":   "http",
				"url.domain":   "www.example.com",
				"url.path":     "/foo",
				"url.port":     77,
				"url.fragment": "bar",
			},
		},
		{
			"query example",
			"https://www.example.com:77/foo?key=val",
			map[string]any{
				"url.original": "https://www.example.com:77/foo?key=val",
				"url.scheme":   "https",
				"url.domain":   "www.example.com",
				"url.path":     "/foo",
				"url.port":     77,
				"url.query":    "key=val",
			},
		},
		{
			"user info",
			"https://user:pw@www.example.com:77/foo",
			map[string]any{
				"url.original":       "https://user:pw@www.example.com:77/foo",
				"url.scheme":         "https",
				"url.domain":         "www.example.com",
				"url.path":           "/foo",
				"url.port":           77,
				AttributeURLUserInfo: "user:pw",
				AttributeURLUsername: "user",
				AttributeURLPassword: "pw",
			},
		},
		{
			"user info - no password",
			"https://user:@www.example.com:77/foo",
			map[string]any{
				"url.original":       "https://user:@www.example.com:77/foo",
				"url.scheme":         "https",
				"url.domain":         "www.example.com",
				"url.path":           "/foo",
				"url.port":           77,
				AttributeURLUserInfo: "user:",
				AttributeURLUsername: "user",
				AttributeURLPassword: "",
			},
		},
		{
			"non-http scheme: ftp",
			"ftp://ftp.is.co.za/rfc/rfc1808.txt",
			map[string]any{
				"url.original":  "ftp://ftp.is.co.za/rfc/rfc1808.txt",
				"url.scheme":    "ftp",
				"url.path":      "/rfc/rfc1808.txt",
				"url.extension": "txt",
				"url.domain":    "ftp.is.co.za",
			},
		},
		{
			"non-http scheme: telnet",
			"telnet://192.0.2.16:80/",
			map[string]any{
				"url.original": "telnet://192.0.2.16:80/",
				"url.scheme":   "telnet",
				"url.path":     "/",
				"url.port":     80,
				"url.domain":   "192.0.2.16",
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
