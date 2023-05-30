// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestParseValidTimestampRaw(t *testing.T) {
	raw := EventRaw{
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
	}
	timestamp := raw.parseTimestamp()
	expected, _ := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.Equal(t, expected, timestamp)
}

func TestParseInvalidTimestampRaw(t *testing.T) {
	raw := EventRaw{
		TimeCreated: TimeCreated{
			SystemTime: "invalid",
		},
	}
	timestamp := raw.parseTimestamp()
	require.Equal(t, time.Now().Year(), timestamp.Year())
	require.Equal(t, time.Now().Month(), timestamp.Month())
	require.Equal(t, time.Now().Day(), timestamp.Day())
}

func TestParseSeverityRaw(t *testing.T) {
	rawRenderedCritical := EventRaw{RenderedLevel: "Critical"}
	rawRenderedError := EventRaw{RenderedLevel: "Error"}
	rawRenderedWarning := EventRaw{RenderedLevel: "Warning"}
	rawRenderedInformation := EventRaw{RenderedLevel: "Information"}
	rawRenderedUnknown := EventRaw{RenderedLevel: "Unknown"}
	rawCritical := EventRaw{Level: "1"}
	rawError := EventRaw{Level: "2"}
	rawWarning := EventRaw{Level: "3"}
	rawInformation := EventRaw{Level: "4"}
	rawUnknown := EventRaw{Level: "0"}
	require.Equal(t, entry.Fatal, rawRenderedCritical.parseRenderedSeverity())
	require.Equal(t, entry.Error, rawRenderedError.parseRenderedSeverity())
	require.Equal(t, entry.Warn, rawRenderedWarning.parseRenderedSeverity())
	require.Equal(t, entry.Info, rawRenderedInformation.parseRenderedSeverity())
	require.Equal(t, entry.Default, rawRenderedUnknown.parseRenderedSeverity())
	require.Equal(t, entry.Fatal, rawCritical.parseRenderedSeverity())
	require.Equal(t, entry.Error, rawError.parseRenderedSeverity())
	require.Equal(t, entry.Warn, rawWarning.parseRenderedSeverity())
	require.Equal(t, entry.Info, rawInformation.parseRenderedSeverity())
	require.Equal(t, entry.Default, rawUnknown.parseRenderedSeverity())
}

func TestParseBodyRaw(t *testing.T) {
	raw := EventRaw{
		bytes: []byte("foo"),
	}

	require.Equal(t, []byte("foo"), raw.parseBody())
}

func TestInvalidUnmarshalRaw(t *testing.T) {
	_, err := unmarshalEventRaw([]byte("Test \n Invalid \t Unmarshal"))
	require.Error(t, err)

}

func TestUnmarshalRaw(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSample.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventRaw(data)
	require.NoError(t, err)

	raw := EventRaw{
		TimeCreated: TimeCreated{
			SystemTime: "2022-04-22T10:20:52.3778625Z",
		},
		Level: "4",
		bytes: data,
	}

	require.Equal(t, raw, event)
}
