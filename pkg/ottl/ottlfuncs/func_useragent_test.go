// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const semconvAttributeUserAgentOSName = "user_agent." + semconv.AttributeOSName
const semconvAttributeUserAgentOSVersion = "user_agent." + semconv.AttributeOSVersion

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
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (Linux; Android 4.1.1; SPH-L710 Build/JRO03L) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 Mobile Safari/535.19",
				semconv.AttributeUserAgentName:     "Chrome Mobile",
				semconv.AttributeUserAgentVersion:  "18.0.1025",
				semconvAttributeUserAgentOSName:    "Android",
				semconvAttributeUserAgentOSVersion: "4.1.1",
			},
		},
		{
			Name:     "Firefox",
			UAString: "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
				semconv.AttributeUserAgentName:     "Firefox",
				semconv.AttributeUserAgentVersion:  "126.0",
				semconvAttributeUserAgentOSName:    "Linux",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "Chrome",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
				semconv.AttributeUserAgentName:     "Chrome",
				semconv.AttributeUserAgentVersion:  "51.0.2704",
				semconvAttributeUserAgentOSName:    "Linux",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "Mobile Safari",
			UAString: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
				semconv.AttributeUserAgentName:     "Mobile Safari",
				semconv.AttributeUserAgentVersion:  "13.1.1",
				semconvAttributeUserAgentOSName:    "iOS",
				semconvAttributeUserAgentOSVersion: "13.5.1",
			},
		},
		{
			Name:     "Edge",
			UAString: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
				semconv.AttributeUserAgentName:     "Edge",
				semconv.AttributeUserAgentVersion:  "91.0.864",
				semconvAttributeUserAgentOSName:    "Windows",
				semconvAttributeUserAgentOSVersion: "10",
			},
		},
		{
			Name:     "Opera",
			UAString: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				semconv.AttributeUserAgentName:     "Opera",
				semconv.AttributeUserAgentVersion:  "38.0.2220",
				semconvAttributeUserAgentOSName:    "Linux",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "curl",
			UAString: "curl/7.81.0",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "curl/7.81.0",
				semconv.AttributeUserAgentName:     "curl",
				semconv.AttributeUserAgentVersion:  "7.81.0",
				semconvAttributeUserAgentOSName:    "Other",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "Unknown user agent",
			UAString: "foobar/1.2.3 (foo; bar baz)",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "foobar/1.2.3 (foo; bar baz)",
				semconv.AttributeUserAgentName:     "Other",
				semconv.AttributeUserAgentVersion:  "",
				semconvAttributeUserAgentOSName:    "Other",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "Otel collector 0.106.1 linux/amd64 user agent",
			UAString: "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "OpenTelemetry Collector Contrib/0.106.1 (linux/amd64)",
				semconv.AttributeUserAgentName:     "Other",
				semconv.AttributeUserAgentVersion:  "",
				semconvAttributeUserAgentOSName:    "Linux",
				semconvAttributeUserAgentOSVersion: "",
			},
		},
		{
			Name:     "ViaFree iOS",
			UAString: "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "ViaFree-DK/3.8.3 (com.MTGx.ViaFree.dk; build:7383; iOS 12.1.0) Alamofire/4.7.0",
				semconv.AttributeUserAgentName:     "ViaFree",
				semconv.AttributeUserAgentVersion:  "3.8.3",
				semconvAttributeUserAgentOSName:    "iOS",
				semconvAttributeUserAgentOSVersion: "12.1.0",
			},
		},
		{
			Name:     "Java SDK Linux",
			UAString: "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
			ExpectedMap: map[string]any{
				semconv.AttributeUserAgentOriginal: "ibm-cos-sdk-java/2.3.0 Linux/4.9.0-8-amd64 Java_HotSpot(TM)_64-Bit_Server_VM/9.0.4+11/9.0.4'",
				semconv.AttributeUserAgentName:     "ibm-cos-sdk-java",
				semconv.AttributeUserAgentVersion:  "2.3.0",
				semconvAttributeUserAgentOSName:    "Linux",
				semconvAttributeUserAgentOSVersion: "4.9.0",
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
