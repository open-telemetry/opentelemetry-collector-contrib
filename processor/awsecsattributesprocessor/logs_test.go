// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func logsWith(attr, value string) plog.Logs {
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr(attr, value)
	return ld
}

func TestConsumeLogs(t *testing.T) {
	srv := newMetadataServer(t)

	tests := []struct {
		name     string
		cfg      *Config
		record   plog.Logs
		wantLen  int
		wantAttr string
		wantVal  string
	}{
		{
			name:   "collect all attributes by default",
			cfg:    &Config{CacheTTL: 60, ContainerID: ContainerID{Sources: []string{"container.id"}}},
			record: logsWith("container.id", testContainerID),
			// The source attribute is container.id, which the enriched container.id
			// (mapped from the Docker ID) overwrites rather than adds.
			wantLen:  len(expectedFlattenedMetadata),
			wantAttr: "aws.ecs.cluster",
			wantVal:  "cds-305",
		},
		{
			name:     "filter to aws attributes",
			cfg:      &Config{CacheTTL: 60, Attributes: []string{"^aws.*"}, ContainerID: ContainerID{Sources: []string{"container.id"}}},
			record:   logsWith("container.id", testContainerID),
			wantLen:  7 + 1, // 7 aws.* keys + container.id
			wantAttr: "aws.ecs.task.arn",
			wantVal:  "arn:aws:ecs:eu-west-1:035955823396:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
		},
		{
			name:     "container id read from log.file.name with suffix",
			cfg:      &Config{CacheTTL: 60, ContainerID: ContainerID{Sources: []string{"log.file.name"}}},
			record:   logsWith("log.file.name", testContainerID+"-json.log"),
			wantLen:  len(expectedFlattenedMetadata) + 1, // + log.file.name source
			wantAttr: "container.image.name",
			wantVal:  "gcr.io/cadvisor/cadvisor:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newLogsProcessor(zaptestLogger(t), tt.cfg, consumertest.NewNop(), staticEndpoints(srv.URL))
			require.NoError(t, tt.cfg.init())

			require.NoError(t, p.ConsumeLogs(t.Context(), tt.record))

			attrs := tt.record.ResourceLogs().At(0).Resource().Attributes()
			require.Equal(t, tt.wantLen, attrs.Len())
			v, ok := attrs.Get(tt.wantAttr)
			require.True(t, ok)
			require.Equal(t, tt.wantVal, v.AsString())
		})
	}
}

func TestConsumeLogsNoContainerID(t *testing.T) {
	srv := newMetadataServer(t)
	cfg := defaultTestConfig()
	require.NoError(t, cfg.init())
	p := newLogsProcessor(zaptestLogger(t), cfg, consumertest.NewNop(), staticEndpoints(srv.URL))

	// No container.id attribute: nothing to enrich, no error.
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("unrelated", "x")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	require.Equal(t, 1, ld.ResourceLogs().At(0).Resource().Attributes().Len())
}
