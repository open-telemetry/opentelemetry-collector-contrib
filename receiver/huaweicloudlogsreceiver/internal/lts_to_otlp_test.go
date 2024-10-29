// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertLTSLogsToOTLP(t *testing.T) {
	tests := []struct {
		name       string
		projectID  string
		regionID   string
		groupID    string
		streamID   string
		ltsLogs    []model.LogContents
		expectLogs int
	}{
		{
			name:      "Valid log content with timestamp",
			projectID: "project1",
			regionID:  "region1",
			groupID:   "group1",
			streamID:  "stream1",
			ltsLogs: []model.LogContents{
				{
					Content: stringPointer("2020-07-25/14:40:00 this log is Error NO 2"),
					LineNum: stringPointer("123"),
					Labels: map[string]string{
						"hostName":      "ecs-kwxtest",
						"hostIP":        "192.168.0.156",
						"appName":       "default_appname",
						"containerName": "CONFIG_FILE",
						"clusterName":   "CONFIG_FILE",
						"hostId":        "9787ef31-f171-4eff-ba71-72d580f11f60",
						"podName":       "default_procname",
						"clusterId":     "CONFIG_FILE",
						"nameSpace":     "CONFIG_FILE",
						"category":      "LTS",
					},
				},
				{
					Content: stringPointer("2020-07-25/14:50:00 this log is Error NO 2"),
					LineNum: stringPointer("456"),
					Labels: map[string]string{
						"hostName":      "ecs-kwxtest",
						"hostIP":        "192.168.0.156",
						"appName":       "default_appname",
						"containerName": "CONFIG_FILE",
						"clusterName":   "CONFIG_FILE",
						"hostId":        "9787ef31-f171-4eff-ba71-72d580f11f60",
						"podName":       "default_procname",
						"clusterId":     "CONFIG_FILE",
						"nameSpace":     "CONFIG_FILE",
						"category":      "LTS",
					},
				},
			},
			expectLogs: 2,
		},
		{
			name:      "Log content without timestamp",
			projectID: "project2",
			regionID:  "region2",
			groupID:   "group2",
			streamID:  "stream2",
			ltsLogs: []model.LogContents{
				{
					Content: stringPointer("this log has no timestamp"),
					Labels:  map[string]string{"level": "info"},
					LineNum: stringPointer("456"),
				},
			},
			expectLogs: 1,
		},
		{
			name:      "Empty log content",
			projectID: "project3",
			regionID:  "region3",
			groupID:   "group3",
			streamID:  "stream3",
			ltsLogs: []model.LogContents{
				{
					Content: stringPointer(""),
					Labels:  map[string]string{"level": "debug"},
					LineNum: stringPointer("789"),
				},
			},
			expectLogs: 1,
		},
		{
			name:      "Multiple logs",
			projectID: "project4",
			regionID:  "region4",
			groupID:   "group4",
			streamID:  "stream4",
			ltsLogs: []model.LogContents{
				{
					Content: stringPointer("2020-07-25/14:44:43 log1"),
					Labels:  map[string]string{"level": "error"},
					LineNum: stringPointer("1"),
				},
				{
					Content: stringPointer("2020-07-25/14:45:43 log2"),
					Labels:  map[string]string{"level": "info"},
					LineNum: stringPointer("2"),
				},
			},
			expectLogs: 2,
		},
		{
			name:      "Invalid timestamp format",
			projectID: "project5",
			regionID:  "region5",
			groupID:   "group5",
			streamID:  "stream5",
			ltsLogs: []model.LogContents{
				{
					Content: stringPointer("25/07/2020 14:44:43 this log has an invalid timestamp"),
					Labels:  map[string]string{"level": "error"},
					LineNum: stringPointer("123"),
				},
			},
			expectLogs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := ConvertLTSLogsToOTLP(tt.projectID, tt.regionID, tt.groupID, tt.streamID, tt.ltsLogs)

			require.Equal(t, 1, logs.ResourceLogs().Len())
			require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().Len())

			logRecords := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			assert.Equal(t, tt.expectLogs, logRecords.Len(), "unexpected number of logs")

			for i := 0; i < tt.expectLogs; i++ {
				logRecord := logRecords.At(i)
				if tt.ltsLogs[i].Content != nil {
					content := *tt.ltsLogs[i].Content
					assert.Equal(t, content, logRecord.Body().AsString())
				}
				// Note: +1 is used because we also add "lineNum" as attribute to the los
				assert.Equal(t, logRecord.Attributes().Len(), len(tt.ltsLogs[i].Labels)+1)
				for key, value := range tt.ltsLogs[i].Labels {
					attr, found := logRecord.Attributes().Get(key)
					assert.True(t, found, "expected attribute %s not found", key)
					assert.Equal(t, value, attr.Str(), "unexpected value for attribute %s", key)
				}
			}
		})
	}
}

func stringPointer(s string) *string {
	return &s
}
