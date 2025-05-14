// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestUserAgentParser(t *testing.T) {
	testCases := []struct {
		Name        string
		UAString    string
		ExpectedMap map[string]any
	}{
		{
			Name:     "Firefox",
			UAString: "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
				string(semconv.UserAgentNameKey):     "Firefox",
				string(semconv.UserAgentVersionKey):  "126.0",
			},
		},
		{
			Name:     "Chrome",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
				string(semconv.UserAgentNameKey):     "Chrome",
				string(semconv.UserAgentVersionKey):  "51.0.2704",
			},
		},
		{
			Name:     "Mobile Safari",
			UAString: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
				string(semconv.UserAgentNameKey):     "Mobile Safari",
				string(semconv.UserAgentVersionKey):  "13.1.1",
			},
		},
		{
			Name:     "Edge",
			UAString: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
				string(semconv.UserAgentNameKey):     "Edge",
				string(semconv.UserAgentVersionKey):  "91.0.864",
			},
		},
		{
			Name:     "Opera",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				string(semconv.UserAgentNameKey):     "Opera",
				string(semconv.UserAgentVersionKey):  "38.0.2220",
			},
		},
		{
			Name:     "curl",
			UAString: "curl/7.81.0",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "curl/7.81.0",
				string(semconv.UserAgentNameKey):     "curl",
				string(semconv.UserAgentVersionKey):  "7.81.0",
			},
		},
		{
			Name:     "Unknown user agent",
			UAString: "foobar/1.2.3 (foo; bar baz)",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "foobar/1.2.3 (foo; bar baz)",
				string(semconv.UserAgentNameKey):     "Other",
				string(semconv.UserAgentVersionKey):  "",
			},
		},
		{
			Name:     "Otel collector 0.106.1 linux/amd64 user agent",
			UAString: "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
			ExpectedMap: map[string]any{
				string(semconv.UserAgentOriginalKey): "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
				string(semconv.UserAgentNameKey):     "Other",
				string(semconv.UserAgentVersionKey):  "",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			source := &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.UAString, nil
				},
			}

			exprFunc := userAgent[any](source) //revive:disable-line:var-naming
			res, err := exprFunc(context.Background(), nil)
			require.NoError(t, err)
			require.IsType(t, map[string]any{}, res)
			resMap := res.(map[string]any)
			assert.Equal(t, tt.ExpectedMap, resMap)
			assert.Len(t, resMap, len(tt.ExpectedMap))
			for k, v := range tt.ExpectedMap {
				if assert.Containsf(t, resMap, k, "key not found %q", k) {
					assert.Equal(t, v, resMap[k])
				}
			}
		})
	}
}
