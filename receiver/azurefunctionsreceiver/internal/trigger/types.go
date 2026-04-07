// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trigger // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

// ParsedRequest holds the decoded invoke payload and trigger metadata for a single binding.
// Content is the list of raw message bodies (e.g. after transport decode); Metadata is
// trigger-specific attributes to add to resources (e.g. Event Hub partition context).
type ParsedRequest struct {
	Content  [][]byte
	Metadata map[string]string
}

// Consumer processes a parsed invoke request and forwards telemetry to the pipeline.
// Each trigger type (e.g. Event Hub logs) has its own implementation in internal packages.
type Consumer interface {
	ConsumeEvents(ctx context.Context, req ParsedRequest) error
}

// AddMetadataToLogs sets the given attributes on every resource in logs.
// Shared by consumers that attach trigger metadata to log resources.
func AddMetadataToLogs(logs *plog.Logs, attrs map[string]string) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		res := logs.ResourceLogs().At(i).Resource()
		for k, v := range attrs {
			res.Attributes().PutStr(k, v)
		}
	}
}
