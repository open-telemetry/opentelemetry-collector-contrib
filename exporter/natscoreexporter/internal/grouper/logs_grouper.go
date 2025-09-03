// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type logsGrouper struct {
	valueExpression *ottl.ValueExpression[ottllog.TransformContext]
}

func (g *logsGrouper) Group(ctx context.Context, srcLogs plog.Logs) ([]Group[plog.Logs], error) {
	var errs error

	type destContext struct {
		logs            plog.Logs
		srcResourceLogs plog.ResourceLogs
		srcScopeLogs    plog.ScopeLogs
	}
	destBySubject := make(map[string]*destContext)

	for _, srcResourceLogs := range srcLogs.ResourceLogs().All() {
		var (
			srcResource       = srcResourceLogs.Resource()
			srcResourceSchema = srcResourceLogs.SchemaUrl()
		)

		for _, srcScopeLogs := range srcResourceLogs.ScopeLogs().All() {
			var (
				srcScope       = srcScopeLogs.Scope()
				srcScopeSchema = srcScopeLogs.SchemaUrl()
			)

			for _, srcLogRecord := range srcScopeLogs.LogRecords().All() {
				transformContext := ottllog.NewTransformContext(
					srcLogRecord,
					srcScope,
					srcResource,
					srcScopeLogs,
					srcResourceLogs,
				)

				subjectAsAny, err := g.valueExpression.Eval(ctx, transformContext)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				subject := subjectAsAny.(string)

				dest, ok := destBySubject[subject]
				if !ok {
					dest = &destContext{
						logs: plog.NewLogs(),
					}
					destBySubject[subject] = dest
				}
				destLogs := dest.logs

				destResourceLogsSlice := destLogs.ResourceLogs()
				if dest.srcResourceLogs != srcResourceLogs {
					dest.srcResourceLogs = srcResourceLogs

					destResourceLogs := destResourceLogsSlice.AppendEmpty()
					srcResource.CopyTo(destResourceLogs.Resource())
					destResourceLogs.SetSchemaUrl(srcResourceSchema)
				}
				destResourceLogs := destResourceLogsSlice.At(destResourceLogsSlice.Len() - 1)

				destScopeLogsSlice := destResourceLogs.ScopeLogs()
				if dest.srcScopeLogs != srcScopeLogs {
					dest.srcScopeLogs = srcScopeLogs

					destScopeLogs := destScopeLogsSlice.AppendEmpty()
					srcScope.CopyTo(destScopeLogs.Scope())
					destScopeLogs.SetSchemaUrl(srcScopeSchema)
				}
				destScopeLogs := destScopeLogsSlice.At(destScopeLogsSlice.Len() - 1)

				destLogRecordSlice := destScopeLogs.LogRecords()
				srcLogRecord.CopyTo(destLogRecordSlice.AppendEmpty())
			}
		}
	}

	groups := make([]Group[plog.Logs], 0, len(destBySubject))
	for subject, dest := range destBySubject {
		groups = append(groups, Group[plog.Logs]{
			Subject: subject,
			Data:    dest.logs,
		})
	}
	return groups, errs
}

var _ Grouper[plog.Logs] = (*logsGrouper)(nil)

func NewLogsGrouper(subject string, telemetrySettings component.TelemetrySettings) (Grouper[plog.Logs], error) {
	parser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[ottllog.TransformContext](),
		telemetrySettings,
	)
	if err != nil {
		return nil, err
	}

	valueExpression, err := parser.ParseValueExpression(subject)
	if err != nil {
		return nil, err
	}

	return &logsGrouper{
		valueExpression: valueExpression,
	}, nil
}
