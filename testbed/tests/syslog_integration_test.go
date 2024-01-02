// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/tests"

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type expectedDataType struct {
	message        string
	severityNumber plog.SeverityNumber
	severityText   string
	timestamp      pcommon.Timestamp
	attributes     map[string]any
}

func TestSyslogComplementaryRFC5424(t *testing.T) {
	expectedData := []expectedDataType{
		{
			message:        "<165>1 2003-10-11T22:14:15.003Z mymachine.example.com eventslog - ID47 [exampleSDID@32473 iut=\"3\"] Some message",
			severityNumber: 10,
			severityText:   "notice",
			timestamp:      1065910455003000000,
			attributes: map[string]any{
				"message": "Some message",
				"msg_id":  "ID47",
				"structured_data": map[string]any{
					"exampleSDID@32473": map[string]any{
						"iut": "3",
					},
				},
				"hostname": "mymachine.example.com",
				"appname":  "eventslog",
				"priority": int64(165),
				"version":  int64(1),
				"facility": int64(20),
			},
		},
		{
			message:        "<17>3 2003-10-11T22:14:15.008Z - - - - -",
			severityNumber: 19,
			severityText:   "alert",
			timestamp:      1065910455008000000,
			attributes: map[string]any{
				"priority": int64(17),
				"version":  int64(3),
				"facility": int64(2),
			},
		},
	}

	complementaryTest(t, "rfc5424", expectedData)
}

func TestSyslogComplementaryRFC3164(t *testing.T) {
	expectedData := []expectedDataType{
		{
			message:        "<34>Oct 11 22:14:15 mymachine su: 'su root' failed for lonvick on /dev/pts/8",
			timestamp:      1697062455000000000,
			severityNumber: 18,
			severityText:   "crit",
			attributes: map[string]any{
				"message":  "'su root' failed for lonvick on /dev/pts/8",
				"hostname": "mymachine",
				"appname":  "su",
				"priority": int64(34),
				"facility": int64(4),
			},
		},
		{
			message:        "<19>Oct 11 22:14:15 - -",
			timestamp:      1697062455000000000,
			severityNumber: 17,
			severityText:   "err",
			attributes: map[string]any{
				"message":  "-",
				"priority": int64(19),
				"facility": int64(2),
			},
		},
	}

	complementaryTest(t, "rfc3164", expectedData)
}

func componentFactories(t *testing.T) otelcol.Factories {
	factories, err := testbed.Components()
	require.NoError(t, err)
	return factories
}

func complementaryTest(t *testing.T, rfc string, expectedData []expectedDataType) {
	// Prepare ports
	port := testbed.GetAvailablePort(t)
	inputPort := testbed.GetAvailablePort(t)

	// Start SyslogDataReceiver
	syslogReceiver := datareceivers.NewSyslogDataReceiver(rfc, port)
	backend := testbed.NewMockBackend("mockbackend.log", syslogReceiver)
	require.NoError(t, backend.Start())
	backend.EnableRecording()

	// Prepare and run collector
	config := `
receivers:
  syslog/client:
    protocol: %s
    tcp:
      listen_address: '127.0.0.1:%d'
exporters:
  syslog/client:
    endpoint: 127.0.0.1
    network: tcp
    protocol: %s
    port: %d
    tls:
      insecure: true
service:
  pipelines:
    logs/client:
      receivers:
        - syslog/client
      exporters:
        - syslog/client`

	collector := testbed.NewInProcessCollector(componentFactories(t))
	_, err := collector.PrepareConfig(fmt.Sprintf(config, rfc, inputPort, rfc, port))

	require.NoError(t, err)
	err = collector.Start(testbed.StartParams{
		Name: "Agent",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		stopped, e := collector.Stop()
		require.NoError(t, e)
		require.True(t, stopped)
	})

	// prepare data

	message := ""
	expectedAttributes := []map[string]any{}
	expectedLogs := plog.NewLogs()
	rl := expectedLogs.ResourceLogs().AppendEmpty()
	lrs := rl.ScopeLogs().AppendEmpty().LogRecords()

	for _, e := range expectedData {
		lr := lrs.AppendEmpty()
		lr.Body().SetStr(e.message)
		lr.SetSeverityNumber(e.severityNumber)
		lr.SetSeverityText(e.severityText)
		lr.SetTimestamp(e.timestamp)
		expectedAttributes = append(expectedAttributes, e.attributes)
		message += e.message + "\n"
	}

	// Prepare client
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", inputPort))
	require.NoError(t, err)

	// Write requests
	fmt.Fprint(conn, message)

	// Wait for all messages
	for len(backend.ReceivedLogs) < 1 {
		time.Sleep(100 * time.Millisecond)
	}

	require.Equal(t, len(backend.ReceivedLogs), 1)
	require.Equal(t, backend.ReceivedLogs[0].ResourceLogs().Len(), 1)
	require.Equal(t, backend.ReceivedLogs[0].ResourceLogs().At(0).ScopeLogs().Len(), 1)
	require.Equal(t, backend.ReceivedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len(), len(expectedData))

	// Clean received logs
	attributes := []map[string]any{}

	lrs = backend.ReceivedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	for i := 0; i < lrs.Len(); i++ {
		lrs.At(i).SetObservedTimestamp(0)

		attributes = append(attributes, lrs.At(i).Attributes().AsRaw())
		lrs.At(i).Attributes().Clear()
	}

	// Assert
	assert.Equal(t, expectedLogs, backend.ReceivedLogs[0])
	assert.Equal(t, expectedAttributes, attributes)
}
