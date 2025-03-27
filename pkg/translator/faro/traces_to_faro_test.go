// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

func TestTranslateFromTraces(t *testing.T) {
	testcases := []struct {
		name         string
		ptracesFile  string
		wantPayloads []faroTypes.Payload
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:         "Empty traces",
			ptracesFile:  filepath.Join("testdata", "empty-payload", "ptraces.yaml"),
			wantPayloads: []faroTypes.Payload{},
			wantErr:      assert.NoError,
		},
		{
			name:         "Empty scope spans and empty resource attribute",
			ptracesFile:  filepath.Join("testdata", "empty-scope-empty-resource-attributes-ptraces.yaml"),
			wantPayloads: []faroTypes.Payload{},
			wantErr:      assert.NoError,
		},
		{
			name:        "Two resource spans with different resource attributes should produce two faro payloads",
			ptracesFile: filepath.Join("testdata", "two-resource-spans-different-resource-attributes-ptraces", "ptraces.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "two-resource-spans-different-resource-attributes-ptraces/payload-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "two-resource-spans-different-resource-attributes-ptraces/payload-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:        "Two resource spans with the same resource attributes should produce one faro payload",
			ptracesFile: filepath.Join("testdata", "two-resource-spans-same-resource-attributes-ptraces", "ptraces.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "two-resource-spans-same-resource-attributes-ptraces/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			ptraces, err := golden.ReadTraces(tt.ptracesFile)
			require.NoError(t, err)
			payloads, err := TranslateFromTraces(context.TODO(), ptraces)
			tt.wantErr(t, err)
			assert.ElementsMatch(t, tt.wantPayloads, payloads)
		})
	}
}

func Test_extractMetaFromResourceAttributes(t *testing.T) {
	testcases := []struct {
		name               string
		resourceAttributes pcommon.Map
		wantMeta           faroTypes.Meta
	}{
		{
			name: "Resource attributes contain all the attributes for meta",
			resourceAttributes: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr(string(semconv.ServiceNameKey), "testapp")
				attrs.PutStr(string(semconv.ServiceNamespaceKey), "testnamespace")
				attrs.PutStr(string(semconv.ServiceVersionKey), "1.0.0")
				attrs.PutStr(string(semconv.DeploymentEnvironmentKey), "production")
				attrs.PutStr(faroAppBundleID, "123")
				attrs.PutStr(string(semconv.TelemetrySDKNameKey), "telemetry sdk")
				attrs.PutStr(string(semconv.TelemetrySDKVersionKey), "1.0.0")

				return attrs
			}(),
			wantMeta: faroTypes.Meta{
				App: faroTypes.App{
					Name:        "testapp",
					Namespace:   "testnamespace",
					Version:     "1.0.0",
					Environment: "production",
					BundleID:    "123",
				},
				SDK: faroTypes.SDK{
					Name:    "telemetry sdk",
					Version: "1.0.0",
				},
			},
		},
		{
			name:               "Resource attributes don't contain attributes for meta",
			resourceAttributes: pcommon.NewMap(),
			wantMeta:           faroTypes.Meta{},
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantMeta, extractMetaFromResourceAttributes(tt.resourceAttributes), "extractMetaFromResourceAttributes(%v)", tt.resourceAttributes.AsRaw())
		})
	}
}
