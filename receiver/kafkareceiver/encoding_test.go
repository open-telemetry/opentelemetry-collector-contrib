// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/unmarshaler"
)

var (
	customLogsUnmarshalerExtension struct {
		component.Component
		plog.Unmarshaler
	}
	customMetricsUnmarshalerExtension struct {
		component.Component
		pmetric.Unmarshaler
	}
	customTracesUnmarshalerExtension struct {
		component.Component
		ptrace.Unmarshaler
	}
)

func TestGetLogsUnmarshaler(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify built-in unmarshalers.
	u := mustGetLogsUnmarshaler(t, "otlp_proto", componenttest.NewNopHost())
	assert.Equal(t, &plog.ProtoUnmarshaler{}, u)
	_ = mustGetLogsUnmarshaler(t, "otlp_json", componenttest.NewNopHost())
	_ = mustGetLogsUnmarshaler(t, "raw", componenttest.NewNopHost())
	_ = mustGetLogsUnmarshaler(t, "json", componenttest.NewNopHost())
	_ = mustGetLogsUnmarshaler(t, "azure_resource_logs", componenttest.NewNopHost())

	// Verify extensions take precedence over built-in unmarshalers.
	u = mustGetLogsUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customLogsUnmarshalerExtension,
	})
	assert.Equal(t, &customLogsUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := getLogsUnmarshaler("otlp_proto", settings, extensionsHost{
		component.MustNewID("otlp_proto"): &customTracesUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a logs unmarshaler`)
	assert.Nil(t, u)

	// Special case for text unmarshaler: "text" is a utf-8 text unmarshaler,
	// while "text_<encoding>" is a text unmarshaler with a specific encoding.
	utf8Unmarshaler, err := unmarshaler.NewTextLogsUnmarshaler("utf-8")
	require.NoError(t, err)
	utf16Unmarshaler, err := unmarshaler.NewTextLogsUnmarshaler("utf-16")
	require.NoError(t, err)

	u = mustGetLogsUnmarshaler(t, "text", componenttest.NewNopHost())
	assert.Equal(t, utf8Unmarshaler, u)

	u = mustGetLogsUnmarshaler(t, "text_utf16", componenttest.NewNopHost())
	assert.Equal(t, utf16Unmarshaler, u)

	u, err = getLogsUnmarshaler("text_invalid", settings, componenttest.NewNopHost())
	require.EqualError(t, err, `invalid text encoding: unsupported encoding 'invalid'`)
	assert.Nil(t, u)
}

func TestGetMetricsUnmarshaler(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify a built-in unmarshaler.
	u := mustGetMetricsUnmarshaler(t, "otlp_proto", componenttest.NewNopHost())
	assert.Equal(t, &pmetric.ProtoUnmarshaler{}, u)

	// Verify extensions take precedence over built-in unmarshalers.
	u = mustGetMetricsUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customMetricsUnmarshalerExtension,
	})
	assert.Equal(t, &customMetricsUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := getMetricsUnmarshaler("otlp_proto", settings, extensionsHost{
		component.MustNewID("otlp_proto"): &customLogsUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a metrics unmarshaler`)
	assert.Nil(t, u)
}

func TestGetTracesUnmarshaler(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify a built-in unmarshaler.
	u := mustGetTracesUnmarshaler(t, "otlp_proto", componenttest.NewNopHost())
	assert.Equal(t, &ptrace.ProtoUnmarshaler{}, u)
	_ = mustGetTracesUnmarshaler(t, "jaeger_proto", componenttest.NewNopHost())
	_ = mustGetTracesUnmarshaler(t, "jaeger_json", componenttest.NewNopHost())
	_ = mustGetTracesUnmarshaler(t, "zipkin_proto", componenttest.NewNopHost())
	_ = mustGetTracesUnmarshaler(t, "zipkin_json", componenttest.NewNopHost())
	_ = mustGetTracesUnmarshaler(t, "zipkin_thrift", componenttest.NewNopHost())

	// Verify extensions take precedence over built-in unmarshalers.
	u = mustGetTracesUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customTracesUnmarshalerExtension,
	})
	assert.Equal(t, &customTracesUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := getTracesUnmarshaler("otlp_proto", settings, extensionsHost{
		component.MustNewID("otlp_proto"): &customLogsUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a traces unmarshaler`)
	assert.Nil(t, u)
}

func mustGetLogsUnmarshaler(tb testing.TB, encoding string, host component.Host) plog.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := getLogsUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

func mustGetMetricsUnmarshaler(tb testing.TB, encoding string, host component.Host) pmetric.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := getMetricsUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

func mustGetTracesUnmarshaler(tb testing.TB, encoding string, host component.Host) ptrace.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := getTracesUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

type extensionsHost map[component.ID]component.Component

func (h extensionsHost) GetExtensions() map[component.ID]component.Component {
	return h
}
