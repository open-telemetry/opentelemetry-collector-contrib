// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/require"
)

func TestURIParser(t *testing.T) {
	testCases := []struct {
		Name        string
		Original    string
		ExpectedMap map[string]any
	}{
		{
			"complete example",
			"http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
			map[string]any{
				"path":      "/foo.gif",
				"fragment":  "fragment",
				"extension": "gif",
				"password":  "mypassword",
				"original":  "http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
				"scheme":    "http",
				"port":      80,
				"user_info": "myusername:mypassword",
				"domain":    "www.example.com",
				"query":     "key1=val1&key2=val2",
				"username":  "myusername",
			},
		},
		{
			"simple example",
			"http://www.example.com",
			map[string]any{
				"original": "http://www.example.com",
				"scheme":   "http",
				"domain":   "www.example.com",
				"path":     "",
			},
		},
		{
			"custom port",
			"http://www.example.com:77",
			map[string]any{
				"original": "http://www.example.com:77",
				"scheme":   "http",
				"domain":   "www.example.com",
				"path":     "",
				"port":     77,
			},
		},
		{
			"file",
			"http://www.example.com:77/file.png",
			map[string]any{
				"original":  "http://www.example.com:77/file.png",
				"scheme":    "http",
				"domain":    "www.example.com",
				"path":      "/file.png",
				"port":      77,
				"extension": "png",
			},
		},
		{
			"fragment",
			"http://www.example.com:77/foo#bar",
			map[string]any{
				"original": "http://www.example.com:77/foo#bar",
				"scheme":   "http",
				"domain":   "www.example.com",
				"path":     "/foo",
				"port":     77,
				"fragment": "bar",
			},
		},
		{
			"query example",
			"https://www.example.com:77/foo?key=val",
			map[string]any{
				"original": "https://www.example.com:77/foo?key=val",
				"scheme":   "https",
				"domain":   "www.example.com",
				"path":     "/foo",
				"port":     77,
				"query":    "key=val",
			},
		},
		{
			"user info",
			"https://user:pw@www.example.com:77/foo",
			map[string]any{
				"original":  "https://user:pw@www.example.com:77/foo",
				"scheme":    "https",
				"domain":    "www.example.com",
				"path":      "/foo",
				"port":      77,
				"user_info": "user:pw",
				"username":  "user",
				"password":  "pw",
			},
		},
		{
			"user info - no password",
			"https://user:@www.example.com:77/foo",
			map[string]any{
				"original":  "https://user:@www.example.com:77/foo",
				"scheme":    "https",
				"domain":    "www.example.com",
				"path":      "/foo",
				"port":      77,
				"user_info": "user:",
				"username":  "user",
				"password":  "",
			},
		},
		{
			"non-http scheme: ftp",
			"ftp://ftp.is.co.za/rfc/rfc1808.txt",
			map[string]any{
				"original":  "ftp://ftp.is.co.za/rfc/rfc1808.txt",
				"scheme":    "ftp",
				"path":      "/rfc/rfc1808.txt",
				"extension": "txt",
				"domain":    "ftp.is.co.za",
			},
		},
		{
			"non-http scheme: telnet",
			"telnet://192.0.2.16:80/",
			map[string]any{
				"original": "telnet://192.0.2.16:80/",
				"scheme":   "telnet",
				"path":     "/",
				"port":     80,
				"domain":   "192.0.2.16",
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

			exprFunc, err := URI(source)
			require.NoError(t, err)

			res, err := exprFunc(context.Background(), nil)
			require.NoError(t, err)

			resMap, ok := res.(map[string]any)
			require.True(t, ok)

			require.Equal(t, len(tc.ExpectedMap), len(resMap))
			for k, v := range tc.ExpectedMap {
				actualValue, found := resMap[k]
				require.True(t, found, "key not found %q", k)
				require.Equal(t, v, actualValue)
			}
		})
	}
}
