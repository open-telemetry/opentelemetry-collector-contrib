// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package uri

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	cfg := NewConfigWithID("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("uri_parser")
	require.True(t, ok, "expected uri_parser to be registered")
	require.Equal(t, "uri_parser", builder().Type())
}

func TestParserBuildFailure(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OnError = "invalid_on_error"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]uint8' cannot be parsed as URI")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse \"invalid\": invalid URI for request")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as URI")
}

func TestProcess(t *testing.T) {
	cases := []struct {
		name   string
		op     func() (operator.Operator, error)
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			"default",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: "https://google.com:443/path?user=dev",
			},
			&entry.Entry{
				Attributes: map[string]any{
					"host": "google.com",
					"port": "443",
					"path": "/path",
					"query": map[string]any{
						"user": []any{
							"dev",
						},
					},
					"scheme": "https",
				},
				Body: "https://google.com:443/path?user=dev",
			},
		},
		{
			"parse-to",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.ParseFrom = entry.NewBodyField("url")
				cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField("url2")}
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
					"url2": map[string]any{
						"host": "google.com",
						"port": "443",
						"path": "/path",
						"query": map[string]any{
							"user": []any{
								"dev",
							},
						},
						"scheme": "https",
					},
				},
			},
		},
		{
			"parse-from",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.ParseFrom = entry.NewBodyField("url")
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
				Attributes: map[string]any{
					"host": "google.com",
					"port": "443",
					"path": "/path",
					"query": map[string]any{
						"user": []any{
							"dev",
						},
					},
					"scheme": "https",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.op()
			require.NoError(t, err, "did not expect operator function to return an error, this is a bug with the test case")

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestParserParse(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  any
		outputBody map[string]any
		expectErr  bool
	}{
		{
			"string",
			"http://www.google.com/app?env=prod",
			map[string]any{
				"scheme": "http",
				"host":   "www.google.com",
				"path":   "/app",
				"query": map[string]any{
					"env": []any{
						"prod",
					},
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := Parser{}
			x, err := parser.parse(tc.inputBody)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.outputBody, x)
		})
	}
}

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
			x, err := parseURI(tc.inputBody)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.outputBody, x)
		})
	}
}

func TestBuildParserURL(t *testing.T) {
	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicParser()
		_, err := c.Build(testutil.Logger(t))
		require.NoError(t, err)
	})
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
			require.Equal(t, tc.outputBody, urlToMap(tc.inputBody, m))
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

func BenchmarkParserParse(b *testing.B) {
	v := "https://dev:password@www.golang.org:8443/v1/app/stage?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	parser := Parser{}
	for n := 0; n < b.N; n++ {
		if _, err := parser.parse(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkURLToMap(b *testing.B) {
	m := make(map[string]any)
	v := "https://dev:password@www.golang.org:8443/v1/app/stage?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	u, err := url.ParseRequestURI(v)
	if err != nil {
		b.Fatal(err)
	}
	for n := 0; n < b.N; n++ {
		urlToMap(u, m)
	}
}

func BenchmarkQueryToMap(b *testing.B) {
	m := make(map[string]any)
	v := "?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	u, err := url.ParseQuery(v)
	if err != nil {
		b.Fatal(err)
	}
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
