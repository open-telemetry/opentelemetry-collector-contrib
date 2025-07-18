// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

func (dp *dnsLookupProcessor) processLogs(ctx context.Context, ls plog.Logs) (plog.Logs, error) {
	for _, resourceLogs := range ls.ResourceLogs().All() {
		for _, lookup := range dp.config.Lookups {
			switch lookup.Context {
			case resource:
				err := dp.processLookup(ctx, resourceLogs.Resource().Attributes(), lookup)
				if err != nil {
					return ls, err
				}
			case record:
				for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
					for _, logRecords := range scopeLogs.LogRecords().All() {
						err := dp.processLookup(ctx, logRecords.Attributes(), lookup)
						if err != nil {
							return ls, err
						}
					}
				}
			default:
				// This should never happen, as config validation should ensure the context is valid.
				return ls, nil
			}
		}
	}

	return ls, nil
}
