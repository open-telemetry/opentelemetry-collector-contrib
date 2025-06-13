// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"

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
				semconv.AttributeURLPath:      "/foo.gif",
				semconv.AttributeURLFragment:  "fragment",
				semconv.AttributeURLExtension: "gif",
				AttributeURLPassword:          "mypassword",
				semconv.AttributeURLOriginal:  "http://myusername:mypassword@www.example.com:80/foo.gif?key1=val1&key2=val2#fragment",
				semconv.AttributeURLScheme:    "http",
				semconv.AttributeURLPort:      80,
				AttributeURLUserInfo:          "myusername:mypassword",
				semconv.AttributeURLDomain:    "www.example.com",
				semconv.AttributeURLQuery:     "key1=val1&key2=val2",
				AttributeURLUsername:          "myusername",
			},
		},
		{
			"simple example",
			"http://www.example.com",
			map[string]any{
				semconv.AttributeURLOriginal: "http://www.example.com",
				semconv.AttributeURLScheme:   "http",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "",
			},
		},
		{
			"custom port",
			"http://www.example.com:77",
			map[string]any{
				semconv.AttributeURLOriginal: "http://www.example.com:77",
				semconv.AttributeURLScheme:   "http",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "",
				semconv.AttributeURLPort:     77,
			},
		},
		{
			"file",
			"http://www.example.com:77/file.png",
			map[string]any{
				semconv.AttributeURLOriginal:  "http://www.example.com:77/file.png",
				semconv.AttributeURLScheme:    "http",
				semconv.AttributeURLDomain:    "www.example.com",
				semconv.AttributeURLPath:      "/file.png",
				semconv.AttributeURLPort:      77,
				semconv.AttributeURLExtension: "png",
			},
		},
		{
			"fragment",
			"http://www.example.com:77/foo#bar",
			map[string]any{
				semconv.AttributeURLOriginal: "http://www.example.com:77/foo#bar",
				semconv.AttributeURLScheme:   "http",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "/foo",
				semconv.AttributeURLPort:     77,
				semconv.AttributeURLFragment: "bar",
			},
		},
		{
			"query example",
			"https://www.example.com:77/foo?key=val",
			map[string]any{
				semconv.AttributeURLOriginal: "https://www.example.com:77/foo?key=val",
				semconv.AttributeURLScheme:   "https",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "/foo",
				semconv.AttributeURLPort:     77,
				semconv.AttributeURLQuery:    "key=val",
			},
		},
		{
			"user info",
			"https://user:pw@www.example.com:77/foo",
			map[string]any{
				semconv.AttributeURLOriginal: "https://user:pw@www.example.com:77/foo",
				semconv.AttributeURLScheme:   "https",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "/foo",
				semconv.AttributeURLPort:     77,
				AttributeURLUserInfo:         "user:pw",
				AttributeURLUsername:         "user",
				AttributeURLPassword:         "pw",
			},
		},
		{
			"user info - no password",
			"https://user:@www.example.com:77/foo",
			map[string]any{
				semconv.AttributeURLOriginal: "https://user:@www.example.com:77/foo",
				semconv.AttributeURLScheme:   "https",
				semconv.AttributeURLDomain:   "www.example.com",
				semconv.AttributeURLPath:     "/foo",
				semconv.AttributeURLPort:     77,
				AttributeURLUserInfo:         "user:",
				AttributeURLUsername:         "user",
				AttributeURLPassword:         "",
			},
		},
		{
			"non-http scheme: ftp",
			"ftp://ftp.is.co.za/rfc/rfc1808.txt",
			map[string]any{
				semconv.AttributeURLOriginal:  "ftp://ftp.is.co.za/rfc/rfc1808.txt",
				semconv.AttributeURLScheme:    "ftp",
				semconv.AttributeURLPath:      "/rfc/rfc1808.txt",
				semconv.AttributeURLExtension: "txt",
				semconv.AttributeURLDomain:    "ftp.is.co.za",
			},
		},
		{
			"non-http scheme: telnet",
			"telnet://192.0.2.16:80/",
			map[string]any{
				semconv.AttributeURLOriginal: "telnet://192.0.2.16:80/",
				semconv.AttributeURLScheme:   "telnet",
				semconv.AttributeURLPath:     "/",
				semconv.AttributeURLPort:     80,
				semconv.AttributeURLDomain:   "192.0.2.16",
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
