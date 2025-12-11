// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestUserAgentParser(t *testing.T) {
	testCases := []struct {
		Name        string
		UAString    string
		ExpectedMap map[string]any
	}{
		{
			Name:     "Firefox-Android",
			UAString: "Mozilla/5.0 (Linux; Android 4.1.1; SPH-L710 Build/JRO03L) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 Mobile Safari/535.19",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (Linux; Android 4.1.1; SPH-L710 Build/JRO03L) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 Mobile Safari/535.19",
				string(conventions.UserAgentNameKey):     "Chrome Mobile",
				string(conventions.UserAgentVersionKey):  "18.0.1025",
				string(conventions.OSNameKey):            "Android",
				string(conventions.OSVersionKey):         "4.1.1",
			},
		},
		{
			Name:     "Firefox",
			UAString: "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
				string(conventions.UserAgentNameKey):     "Firefox",
				string(conventions.UserAgentVersionKey):  "126.0",
				string(conventions.OSNameKey):            "Linux",
			},
		},
		{
			Name:     "Chrome",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
				string(conventions.UserAgentNameKey):     "Chrome",
				string(conventions.UserAgentVersionKey):  "51.0.2704",
				string(conventions.OSNameKey):            "Linux",
			},
		},
		{
			Name:     "Mobile Safari",
			UAString: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
				string(conventions.UserAgentNameKey):     "Mobile Safari",
				string(conventions.UserAgentVersionKey):  "13.1.1",
				string(conventions.OSNameKey):            "iOS",
				string(conventions.OSVersionKey):         "13.5.1",
			},
		},
		{
			Name:     "Edge",
			UAString: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
				string(conventions.UserAgentNameKey):     "Edge",
				string(conventions.UserAgentVersionKey):  "91.0.864",
				string(conventions.OSNameKey):            "Windows",
				string(conventions.OSVersionKey):         "10",
			},
		},
		{
			Name:     "Opera",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				string(conventions.UserAgentNameKey):     "Opera",
				string(conventions.UserAgentVersionKey):  "38.0.2220",
				string(conventions.OSNameKey):            "Linux",
			},
		},
		{
			Name:     "curl",
			UAString: "curl/7.81.0",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "curl/7.81.0",
				string(conventions.UserAgentNameKey):     "curl",
				string(conventions.UserAgentVersionKey):  "7.81.0",
				string(conventions.OSNameKey):            "Other",
			},
		},
		{
			Name:     "Unknown user agent",
			UAString: "foobar/1.2.3 (foo; bar baz)",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "foobar/1.2.3 (foo; bar baz)",
				string(conventions.UserAgentNameKey):     "Other",
				string(conventions.UserAgentVersionKey):  "",
				string(conventions.OSNameKey):            "Other",
			},
		},
		{
			Name:     "Otel collector 0.106.1 linux/amd64 user agent",
			UAString: "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
				string(conventions.UserAgentNameKey):     "Other",
				string(conventions.UserAgentVersionKey):  "",
				string(conventions.OSNameKey):            "Linux",
			},
		},
		{
			Name:     "ViaFree iOS",
			UAString: "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
				string(conventions.UserAgentNameKey):     "ViaFree",
				string(conventions.UserAgentVersionKey):  "3.8.3",
				string(conventions.OSNameKey):            "iOS",
				string(conventions.OSVersionKey):         "12.1.0",
			},
		},
		{
			Name:     "Java SDK Linux",
			UAString: "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
			ExpectedMap: map[string]any{
				string(conventions.UserAgentOriginalKey): "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
				string(conventions.UserAgentNameKey):     "ibm-cos-sdk-java",
				string(conventions.UserAgentVersionKey):  "2.3.0",
				string(conventions.OSNameKey):            "Linux",
				string(conventions.OSVersionKey):         "4.9.0",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			source := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.UAString, nil
				},
			}

			exprFunc := userAgent[any](source) //revive:disable-line:var-naming
			res, err := exprFunc(t.Context(), nil)
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
