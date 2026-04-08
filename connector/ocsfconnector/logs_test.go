// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ocsfconnector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/ocsfconnector/internal/metadata"
)

func TestLogsConnectorConsumeLogs(t *testing.T) {
	for _, test := range []struct {
		name              string
		cfg               Config
		logMessage        string
		expectedOCSFEvent ocsfEvent
	}{
		{
			name: "ssh connection",
			cfg: Config{
				Mappings: []*Mapping{
					{
						Detection:       "(?P<user>\\w+) ssh opened",
						ClassUID:        1,
						ClassName:       "ssh",
						CategoryUID:     1,
						CategoryName:    "intrusion",
						ActivityID:      1,
						ActivityName:    "detection",
						SeverityID:      1,
						Severity:        "err",
						MessageTemplate: "User {{ .user }} opened a SSH connection",
					},
				},
			},
			logMessage: "bob ssh opened",
			expectedOCSFEvent: ocsfEvent{
				Metadata: ocsfMetadata{
					Version: "1",
					Product: ocsfProduct{
						VendorName: "OpenTelemetry",
						Name:       "ocsfconnector",
					},
				},
				ClassUID:     1,
				ClassName:    "ssh",
				CategoryUID:  1,
				CategoryName: "intrusion",
				ActivityID:   1,
				ActivityName: "detection",
				SeverityID:   1,
				Severity:     "err",
				Message:      "User bob opened a SSH connection",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := connectortest.NewNopSettings(metadata.Type)
			sink := &consumertest.LogsSink{}
			c, err := newLogsConnector(set, &test.cfg, sink)
			require.NoError(t, err)

			logs := plog.NewLogs()
			lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			lr.Body().SetStr(test.logMessage)

			err = c.ConsumeLogs(t.Context(), logs)
			require.NoError(t, err)

			assert.Equal(t, 1, sink.LogRecordCount())
			ocsfLR := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			var oe ocsfEvent
			err = json.Unmarshal([]byte(ocsfLR.Body().Str()), &oe)
			require.NoError(t, err)
			oe.Time = test.expectedOCSFEvent.Time
			require.Equal(t, test.expectedOCSFEvent, oe)
		})
	}
}
