// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"
import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestExtension_Start(t *testing.T) {
	tests := []struct {
		name         string
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "otlpJson",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "otlp_json"
				return factory.Create(context.Background(), extensiontest.NewNopSettings(factory.Type()), cfg)
			},
		},

		{
			name: "otlpProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "otlp_proto"
				return factory.Create(context.Background(), extensiontest.NewNopSettings(factory.Type()), cfg)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
			err = ext.Start(context.Background(), componenttest.NewNopHost())
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func testOTLPMarshal(t *testing.T, ex *otlpExtension) {
	traces := generateTraces()
	_, err := ex.MarshalTraces(traces)
	require.NoError(t, err)

	logs := generateLogs()
	_, err = ex.MarshalLogs(logs)
	require.NoError(t, err)

	metrics := generateMetrics()
	_, err = ex.MarshalMetrics(metrics)
	require.NoError(t, err)

	profiles := generateProfiles()
	_, err = ex.MarshalProfiles(profiles)
	require.NoError(t, err)
}

func testOTLPUnmarshal(t *testing.T, ex *otlpExtension) {
	traces := generateTraces()
	logs := generateLogs()
	metrics := generateMetrics()
	profiles := generateProfiles()

	traceBuf, err := ex.MarshalTraces(traces)
	require.NoError(t, err)
	logBuf, err := ex.MarshalLogs(logs)
	require.NoError(t, err)
	metricBuf, err := ex.MarshalMetrics(metrics)
	require.NoError(t, err)
	profileBuf, err := ex.MarshalProfiles(profiles)
	require.NoError(t, err)

	traces0, err := ex.UnmarshalTraces(traceBuf)
	require.NoError(t, err)
	logs0, err := ex.UnmarshalLogs(logBuf)
	require.NoError(t, err)
	metrics0, err := ex.UnmarshalMetrics(metricBuf)
	require.NoError(t, err)
	profiles0, err := ex.UnmarshalProfiles(profileBuf)
	require.NoError(t, err)

	require.Equal(t, traces0.ResourceSpans().Len(), traces.ResourceSpans().Len())
	require.Equal(t, logs0.ResourceLogs().Len(), logs.ResourceLogs().Len())
	require.Equal(t, metrics0.ResourceMetrics().Len(), metrics.ResourceMetrics().Len())
	require.Equal(t, profiles0.ResourceProfiles().Len(), profiles.ResourceProfiles().Len())
}

func TestOTLPJSONMarshal(t *testing.T) {
	conf := &Config{Protocol: otlpJSON}
	ex := createAndExtension0(t, conf)

	testOTLPMarshal(t, ex)
}

func TestOTLPProtoMarshal(t *testing.T) {
	conf := &Config{Protocol: otlpProto}
	ex := createAndExtension0(t, conf)

	testOTLPMarshal(t, ex)
}

func TestOTLPJSONUnmarshal(t *testing.T) {
	conf := &Config{Protocol: otlpJSON}
	ex := createAndExtension0(t, conf)
	testOTLPUnmarshal(t, ex)
}

func TestOTLPProtoUnmarshal(t *testing.T) {
	conf := &Config{Protocol: otlpProto}
	ex := createAndExtension0(t, conf)

	testOTLPUnmarshal(t, ex)
}

// createAndExtension0 Create extension
func createAndExtension0(t *testing.T, c *Config) *otlpExtension {
	ex, err := newExtension(c)
	require.NoError(t, err)
	err = ex.Start(context.TODO(), nil)
	require.NoError(t, err)
	return ex
}

func generateTraces() ptrace.Traces {
	num := 10
	now := time.Now()
	md := ptrace.NewTraces()
	ilm := md.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	ilm.Spans().EnsureCapacity(num)
	for i := 0; i < num; i++ {
		im := ilm.Spans().AppendEmpty()
		im.SetName("test_name")
		im.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
		im.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	}
	return md
}

func generateLogs() plog.Logs {
	num := 10
	md := plog.NewLogs()
	ilm := md.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	ilm.LogRecords().EnsureCapacity(num)
	for i := 0; i < num; i++ {
		im := ilm.LogRecords().AppendEmpty()
		im.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}
	return md
}

func generateMetrics() pmetric.Metrics {
	num := 10
	now := time.Now()
	startTime := pcommon.NewTimestampFromTime(now.Add(-10 * time.Second))
	endTime := pcommon.NewTimestampFromTime(now)

	md := pmetric.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Metrics().EnsureCapacity(num)
	for i := 0; i < num; i++ {
		im := ilm.Metrics().AppendEmpty()
		im.SetName("test_name")
		idp := im.SetEmptySum().DataPoints().AppendEmpty()
		idp.SetStartTimestamp(startTime)
		idp.SetTimestamp(endTime)
		idp.SetIntValue(123)
	}
	return md
}

func generateProfiles() pprofile.Profiles {
	num := 10
	now := time.Now()
	pd := pprofile.NewProfiles()
	ilm := pd.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty()
	ilm.Profiles().EnsureCapacity(num)
	for i := 0; i < num; i++ {
		im := ilm.Profiles().AppendEmpty()
		im.SetProfileID([16]byte{0x01, 0x02, 0x03, 0x04})
		im.SetTime(pcommon.NewTimestampFromTime(now))
		im.SetDuration(pcommon.NewTimestampFromTime(time.Now()))
	}
	return pd
}
