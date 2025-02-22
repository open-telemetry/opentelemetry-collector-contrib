// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parseutils

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

// Test all usecases: absolute uri, relative uri, query string
func TestParseURI(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  string
		outputBody map[string]any
		expectErr  bool
	}{
		{
			"scheme-http",
			"http://",
			map[string]any{
				"scheme": "http",
			},
			false,
		},
		{
			"scheme-user",
			"http://myuser:mypass@",
			map[string]any{
				"scheme": "http",
				"user":   "myuser",
			},
			false,
		},
		{
			"scheme-host",
			"http://golang.com",
			map[string]any{
				"scheme": "http",
				"host":   "golang.com",
			},
			false,
		},
		{
			"scheme-host-root",
			"http://golang.com/",
			map[string]any{
				"scheme": "http",
				"host":   "golang.com",
				"path":   "/",
			},
			false,
		},
		{
			"scheme-host-minimal",
			"http://golang",
			map[string]any{
				"scheme": "http",
				"host":   "golang",
			},
			false,
		},
		{
			"host-missing-scheme",
			"golang.org",
			map[string]any{},
			true,
		},
		{
			"sheme-port",
			"http://:8080",
			map[string]any{
				"scheme": "http",
				"port":   "8080",
			},
			false,
		},
		{
			"port-missing-scheme",
			":8080",
			map[string]any{},
			true,
		},
		{
			"path",
			"/docs",
			map[string]any{
				"path": "/docs",
			},
			false,
		},
		{
			"path-advanced",
			`/x/y%2Fz`,
			map[string]any{
				"path": `/x/y%2Fz`,
			},
			false,
		},
		{
			"path-root",
			"/",
			map[string]any{
				"path": "/",
			},
			false,
		},
		{
			"path-query",
			"/v1/app?user=golang",
			map[string]any{
				"path": "/v1/app",
				"query": map[string]any{
					"user": []any{
						"golang",
					},
				},
			},
			false,
		},
		{
			"invalid-query",
			"?q;go",
			map[string]any{},
			true,
		},
		{
			"scheme-path",
			"http:///v1/app",
			map[string]any{
				"scheme": "http",
				"path":   "/v1/app",
			},
			false,
		},
		{
			"scheme-host-query",
			"https://app.com?token=0000&env=prod&env=stage",
			map[string]any{
				"scheme": "https",
				"host":   "app.com",
				"query": map[string]any{
					"token": []any{
						"0000",
					},
					"env": []any{
						"prod",
						"stage",
					},
				},
			},
			false,
		},
		{
			"minimal",
			"http://golang.org",
			map[string]any{
				"scheme": "http",
				"host":   "golang.org",
			},
			false,
		},
		{
			"advanced",
			"https://go:password@golang.org:8443/v2/app?env=stage&token=456&index=105838&env=prod",
			map[string]any{
				"scheme": "https",
				"user":   "go",
				"host":   "golang.org",
				"port":   "8443",
				"path":   "/v2/app",
				"query": map[string]any{
					"token": []any{
						"456",
					},
					"index": []any{
						"105838",
					},
					"env": []any{
						"stage",
						"prod",
					},
				},
			},
			false,
		},
		{
			"magnet",
			"magnet:?xt=urn:sha1:HNCKHTQCWBTRNJIV4WNAE52SJUQCZO6C",
			map[string]any{
				"scheme": "magnet",
				"query": map[string]any{
					"xt": []any{
						"urn:sha1:HNCKHTQCWBTRNJIV4WNAE52SJUQCZO6C",
					},
				},
			},
			false,
		},
		{
			"sftp",
			"sftp://ftp.com//home/name/employee.csv",
			map[string]any{
				"scheme": "sftp",
				"host":   "ftp.com",
				"path":   "//home/name/employee.csv",
			},
			false,
		},
		{
			"missing-schema",
			"golang.org/app",
			map[string]any{},
			true,
		},
		{
			"query-advanced",
			"?token=0000&env=prod&env=stage&task=update&task=new&action=update",
			map[string]any{
				"query": map[string]any{
					"token": []any{
						"0000",
					},
					"env": []any{
						"prod",
						"stage",
					},
					"task": []any{
						"update",
						"new",
					},
					"action": []any{
						"update",
					},
				},
			},
			false,
		},
		{
			"query",
			"?token=0000",
			map[string]any{
				"query": map[string]any{
					"token": []any{
						"0000",
					},
				},
			},
			false,
		},
		{
			"query-empty",
			"?",
			map[string]any{},
			false,
		},
		{
			"query-empty-key",
			"?user=",
			map[string]any{
				"query": map[string]any{
					"user": []any{
						"", // no value
					},
				},
			},
			false,
		},
		// Query string without a ? prefix is treated as a URI, therefor
		// an error will be returned by url.Parse("user=dev")
		{
			"query-no-?-prefix",
			"user=dev",
			map[string]any{},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, err := ParseURI(tc.inputBody, false)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.outputBody, x)
		})
	}
}

func TestURLToMap(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  *url.URL
		outputBody map[string]any
	}{
		{
			"absolute-uri",
			&url.URL{
				Scheme:   "https",
				Host:     "google.com:8443",
				Path:     "/app",
				RawQuery: "stage=prod&stage=dev",
			},
			map[string]any{
				"scheme": "https",
				"host":   "google.com",
				"port":   "8443",
				"path":   "/app",
				"query": map[string]any{
					"stage": []any{
						"prod",
						"dev",
					},
				},
			},
		},
		{
			"absolute-uri-simple",
			&url.URL{
				Scheme: "http",
				Host:   "google.com",
			},
			map[string]any{
				"scheme": "http",
				"host":   "google.com",
			},
		},
		{
			"path",
			&url.URL{
				Path:     "/app",
				RawQuery: "stage=prod&stage=dev",
			},
			map[string]any{
				"path": "/app",
				"query": map[string]any{
					"stage": []any{
						"prod",
						"dev",
					},
				},
			},
		},
		{
			"path-simple",
			&url.URL{
				Path: "/app",
			},
			map[string]any{
				"path": "/app",
			},
		},
		{
			"query",
			&url.URL{
				RawQuery: "stage=prod&stage=dev",
			},
			map[string]any{
				"query": map[string]any{
					"stage": []any{
						"prod",
						"dev",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := make(map[string]any)
			resMap, err := urlToMap(tc.inputBody, m)
			require.NoError(t, err)
			require.Equal(t, tc.outputBody, resMap)
		})
	}
}

func TestQueryToMap(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  url.Values
		outputBody map[string]any
	}{
		{
			"query",
			url.Values{
				"stage": []string{
					"prod",
					"dev",
				},
			},
			map[string]any{
				"query": map[string]any{
					"stage": []any{
						"prod",
						"dev",
					},
				},
			},
		},
		{
			"empty",
			url.Values{},
			map[string]any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := make(map[string]any)
			require.Equal(t, tc.outputBody, queryToMap(tc.inputBody, m))
		})
	}
}

func TestQueryParamValuesToMap(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  []string
		outputBody []any
	}{
		{
			"simple",
			[]string{
				"prod",
				"dev",
			},
			[]any{
				"prod",
				"dev",
			},
		},
		{
			"empty",
			[]string{},
			[]any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.outputBody, queryParamValuesToMap(tc.inputBody))
		})
	}
}

func BenchmarkURLToMap(b *testing.B) {
	m := make(map[string]any)
	v := "https://dev:password@www.golang.org:8443/v1/app/stage?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	u, err := url.ParseRequestURI(v)
	require.NoError(b, err)
	for n := 0; n < b.N; n++ {
		_, _ = urlToMap(u, m)
	}
}

func BenchmarkQueryToMap(b *testing.B) {
	m := make(map[string]any)
	v := "?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	u, err := url.ParseQuery(v)
	require.NoError(b, err)
	for n := 0; n < b.N; n++ {
		queryToMap(u, m)
	}
}

func BenchmarkQueryParamValuesToMap(b *testing.B) {
	v := []string{
		"d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab",
		"c6fa29f9-a31b-4584-b98d-aa8473b0e18",
	}
	for n := 0; n < b.N; n++ {
		queryParamValuesToMap(v)
	}
}

func TestParseSemconv(t *testing.T) {
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
			resMap, err := ParseURI(tc.Original, true)
			require.NoError(t, err)

			require.Equal(t, len(tc.ExpectedMap), len(resMap))
			for k, v := range tc.ExpectedMap {
				actualValue, found := resMap[k]
				require.True(t, found, "key not found %q", k)
				require.Equal(t, v, actualValue)
			}
		})
	}
}
