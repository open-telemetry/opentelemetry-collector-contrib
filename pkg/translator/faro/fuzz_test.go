// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/pdata/plog"
)

// FuzzTranslateToLogsAndTraces feeds arbitrary Faro payloads, the input the
// Faro receiver takes from browsers, through both translations.
func FuzzTranslateToLogsAndTraces(f *testing.F) {
	payloads, err := filepath.Glob(filepath.Join("testdata", "*", "payload.json"))
	if err != nil {
		f.Fatal(err)
	}
	for _, p := range payloads {
		data, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var payload faroTypes.Payload
		if err := json.Unmarshal(data, &payload); err != nil {
			return
		}
		_, _ = TranslateToLogs(t.Context(), payload)
		_, _ = TranslateToTraces(t.Context(), payload)
	})
}

// FuzzTranslateFromLogs feeds arbitrary log record bodies, the input the Faro
// exporter parses, through the logs to Faro translation.
func FuzzTranslateFromLogs(f *testing.F) {
	f.Add(`kind=log level=info message=hello timestamp=2023-11-16T10:00:55.995Z browser_mobile=false`)
	f.Add(`kind=event event_name=click event_domain=dom event_data_target=button timestamp=2023-11-16T10:00:55.995Z`)
	f.Add(`kind=measurement type=web-vitals value_lcp=1.5 value_cls=0.1 timestamp=2023-11-16T10:00:55.995Z`)
	f.Add("kind=exception type=Error value=boom stacktrace=\"Error: boom\\n  at foo (http://example.com/a.js:1:2)\" browser_brand_0_brand=Chromium browser_brand_0_version=1 sdk_integrations=a:1,b:2 device_is_physical=true k6_isK6Browser=true timestamp=2023-11-16T10:00:55.995Z")

	f.Fuzz(func(t *testing.T, body string) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "fuzz")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(body)
		_, _ = TranslateFromLogs(t.Context(), ld)
	})
}
