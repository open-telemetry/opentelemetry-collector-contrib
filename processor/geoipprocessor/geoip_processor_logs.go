// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

func (g *geoIPProcessor) processLogs(ctx context.Context, ls plog.Logs) (plog.Logs, error) {
	rl := ls.ResourceLogs()
	for i := range rl.Len() {
		switch g.cfg.Context {
		case resource:
			err := g.processAttributes(ctx, rl.At(i).Resource().Attributes())
			if err != nil {
				return ls, err
			}
		case record:
			for j := range rl.At(i).ScopeLogs().Len() {
				for k := range rl.At(i).ScopeLogs().At(j).LogRecords().Len() {
					err := g.processAttributes(ctx, rl.At(i).ScopeLogs().At(j).LogRecords().At(k).Attributes())
					if err != nil {
						return ls, err
					}
				}
			}
		default:
			return ls, errUnspecifiedSource
		}
	}
	return ls, nil
}
