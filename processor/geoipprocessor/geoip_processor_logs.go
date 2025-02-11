// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

func (g *geoIPProcessor) processLogs(ctx context.Context, ls plog.Logs) (plog.Logs, error) {
	rl := ls.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		switch g.cfg.Context {
		case resource:
			err := g.processAttributes(ctx, rl.At(i).Resource().Attributes())
			if err != nil {
				return ls, err
			}
		case record:
			for j := 0; j < rl.At(i).ScopeLogs().Len(); j++ {
				for k := 0; k < rl.At(i).ScopeLogs().At(j).LogRecords().Len(); k++ {
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
