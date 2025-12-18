// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				"user_agent.original": "Mozilla/5.0 (Linux; Android 4.1.1; SPH-L710 Build/JRO03L) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 Mobile Safari/535.19",
				"user_agent.name":     "Chrome Mobile",
				"user_agent.version":  "18.0.1025",
				"os.name":             "Android",
				"os.version":          "4.1.1",
			},
		},
		{
			Name:     "Firefox",
			UAString: "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
			ExpectedMap: map[string]any{
				"user_agent.original": "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
				"user_agent.name":     "Firefox",
				"user_agent.version":  "126.0",
				"os.name":             "Linux",
			},
		},
		{
			Name:     "Chrome",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
			ExpectedMap: map[string]any{
				"user_agent.original": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
				"user_agent.name":     "Chrome",
				"user_agent.version":  "51.0.2704",
				"os.name":             "Linux",
			},
		},
		{
			Name:     "Mobile Safari",
			UAString: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
			ExpectedMap: map[string]any{
				"user_agent.original": "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
				"user_agent.name":     "Mobile Safari",
				"user_agent.version":  "13.1.1",
				"os.name":             "iOS",
				"os.version":          "13.5.1",
			},
		},
		{
			Name:     "Edge",
			UAString: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			ExpectedMap: map[string]any{
				"user_agent.original": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
				"user_agent.name":     "Edge",
				"user_agent.version":  "91.0.864",
				"os.name":             "Windows",
				"os.version":          "10",
			},
		},
		{
			Name:     "Opera",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
			ExpectedMap: map[string]any{
				"user_agent.original": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				"user_agent.name":     "Opera",
				"user_agent.version":  "38.0.2220",
				"os.name":             "Linux",
			},
		},
		{
			Name:     "curl",
			UAString: "curl/7.81.0",
			ExpectedMap: map[string]any{
				"user_agent.original": "curl/7.81.0",
				"user_agent.name":     "curl",
				"user_agent.version":  "7.81.0",
				"os.name":             "Other",
			},
		},
		{
			Name:     "Unknown user agent",
			UAString: "foobar/1.2.3 (foo; bar baz)",
			ExpectedMap: map[string]any{
				"user_agent.original": "foobar/1.2.3 (foo; bar baz)",
				"user_agent.name":     "Other",
				"user_agent.version":  "",
				"os.name":             "Other",
			},
		},
		{
			Name:     "Otel collector 0.106.1 linux/amd64 user agent",
			UAString: "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
			ExpectedMap: map[string]any{
				"user_agent.original": "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
				"user_agent.name":     "Other",
				"user_agent.version":  "",
				"os.name":             "Linux",
			},
		},
		{
			Name:     "ViaFree iOS",
			UAString: "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
			ExpectedMap: map[string]any{
				"user_agent.original": "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
				"user_agent.name":     "ViaFree",
				"user_agent.version":  "3.8.3",
				"os.name":             "iOS",
				"os.version":          "12.1.0",
			},
		},
		{
			Name:     "Java SDK Linux",
			UAString: "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
			ExpectedMap: map[string]any{
				"user_agent.original": "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
				"user_agent.name":     "ibm-cos-sdk-java",
				"user_agent.version":  "2.3.0",
				"os.name":             "Linux",
				"os.version":          "4.9.0",
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
