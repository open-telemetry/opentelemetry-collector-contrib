// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestLogs_RenameAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              plog.Logs
		out             plog.Logs
		transformations string
		targetVersion   string
	}{
		{
			name: "one_version_downgrade",
			in: func() plog.Logs {
				in := plog.NewLogs()
				in.ResourceLogs().AppendEmpty()
				in.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				in.ResourceLogs().At(0).Resource().Attributes().PutStr("new.resource.name", "test-cluster")
				in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
				in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
				in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("new.attr.name", "my-att-cluster")
				return in
			}(),
			out: func() plog.Logs {
				out := plog.NewLogs()
				out.ResourceLogs().AppendEmpty()
				out.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				out.ResourceLogs().At(0).Resource().Attributes().PutStr("old.resource.name", "test-cluster")
				out.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
				out.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
				out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("old.attr.name", "my-att-cluster")

				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  logs:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.attr.name: new.attr.name
	1.8.0:`,
			targetVersion: "1.8.0",
		},
		{
			name: "one_version_upgrade",
			in: func() plog.Logs {
				in := plog.NewLogs()
				in.ResourceLogs().AppendEmpty()
				in.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				in.ResourceLogs().At(0).Resource().Attributes().PutStr("old.resource.name", "test-cluster")
				in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
				in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
				in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("old.attr.name", "my-att-cluster")

				return in
			}(),
			out: func() plog.Logs {
				out := plog.NewLogs()
				out.ResourceLogs().AppendEmpty()
				out.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				out.ResourceLogs().At(0).Resource().Attributes().PutStr("new.resource.name", "test-cluster")
				out.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
				out.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
				out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("new.attr.name", "my-att-cluster")
				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  logs:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.attr.name: new.attr.name
	1.8.0:`,
			targetVersion: "1.9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := newTestSchemaProcessor(t, tt.transformations, tt.targetVersion)
			ctx := context.Background()
			out, err := pr.processLogs(ctx, tt.in)
			if err != nil {
				t.Errorf("Error while processing logs: %v", err)
			}
			assert.Equal(t, tt.out, out, "Logs transformation failed")
		})
	}
}
