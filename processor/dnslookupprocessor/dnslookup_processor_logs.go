// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

func (dp *dnsLookupProcessor) processLogs(ctx context.Context, ls plog.Logs) (plog.Logs, error) {
	for _, resourceLogs := range ls.ResourceLogs().All() {
		for _, pp := range dp.processPairs {
			switch pp.ContextID {
			case resource:
				err := pp.ProcessFn(ctx, resourceLogs.Resource().Attributes())
				if err != nil {
					return ls, err
				}
			case record:
				for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
					for _, logRecords := range scopeLogs.LogRecords().All() {
						err := pp.ProcessFn(ctx, logRecords.Attributes())
						if err != nil {
							return ls, err
						}
					}
				}
			default:
				return ls, errUnknownContextID
			}
		}
	}

	return ls, nil
}
