// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"context"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

func generateLogs(t *testing.T) plog.Logs {
	t.Helper()

	logs := plog.NewLogs()
	for range 10 {
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resourceLogs.Resource().Attributes().PutStr("id", uuid.New().String())
		for range 10 {
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			scopeLogs.Scope().Attributes().PutStr("id", uuid.New().String())
			for range 10 {
				logRecord := scopeLogs.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("id", uuid.New().String())
				logRecord.Attributes().PutStr("subject", strconv.Itoa(rand.Intn(10)))
			}
		}
	}
	return logs
}

type naiveLogsGrouper struct {
	valueExpression *ottl.ValueExpression[ottllog.TransformContext]
}

func (g *naiveLogsGrouper) Group(
	ctx context.Context,
	srcLogs plog.Logs,
) ([]Group[plog.Logs], error) {
	var errs error

	subjectByLogRecord := make(map[plog.LogRecord]string)
	for _, srcResourceLogs := range srcLogs.ResourceLogs().All() {
		for _, srcScopeLogs := range srcResourceLogs.ScopeLogs().All() {
			for _, srcLogRecord := range srcScopeLogs.LogRecords().All() {
				transformContext := ottllog.NewTransformContext(
					srcLogRecord,
					srcScopeLogs.Scope(),
					srcResourceLogs.Resource(),
					srcScopeLogs,
					srcResourceLogs,
				)
				subjectAsAny, err := g.valueExpression.Eval(ctx, transformContext)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				subject := subjectAsAny.(string)
				subjectByLogRecord[srcLogRecord] = subject
			}
		}
	}

	subjects := make(map[string]bool)
	for _, subject := range subjectByLogRecord {
		subjects[subject] = true
	}

	groups := make([]Group[plog.Logs], 0, len(subjects))
	for groupSubject := range subjects {
		destResourceLogsSlice := plog.NewResourceLogsSlice()
		for _, srcResourceLogs := range srcLogs.ResourceLogs().All() {
			destScopeLogsSlice := plog.NewScopeLogsSlice()
			for _, srcScopeLogs := range srcResourceLogs.ScopeLogs().All() {
				destLogRecordSlice := plog.NewLogRecordSlice()
				for _, srcLogRecord := range srcScopeLogs.LogRecords().All() {
					if subjectByLogRecord[srcLogRecord] == groupSubject {
						srcLogRecord.CopyTo(destLogRecordSlice.AppendEmpty())
					}
				}

				if destLogRecordSlice.Len() > 0 {
					destScopeLogs := destScopeLogsSlice.AppendEmpty()
					srcScopeLogs.CopyTo(destScopeLogs)
					destLogRecordSlice.CopyTo(destScopeLogs.LogRecords())
				}
			}

			if destScopeLogsSlice.Len() > 0 {
				destResourceLogs := destResourceLogsSlice.AppendEmpty()
				srcResourceLogs.CopyTo(destResourceLogs)
				destScopeLogsSlice.CopyTo(destResourceLogs.ScopeLogs())
			}
		}

		if destResourceLogsSlice.Len() > 0 {
			destLogs := plog.NewLogs()
			destResourceLogsSlice.CopyTo(destLogs.ResourceLogs())
			groups = append(groups, Group[plog.Logs]{
				Subject: groupSubject,
				Data:    destLogs,
			})
		}
	}
	return groups, errs
}

var _ Grouper[plog.Logs] = &naiveLogsGrouper{}

func newNaiveLogsGrouper(
	subject string,
	telemetrySettings component.TelemetrySettings,
) (Grouper[plog.Logs], error) {
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

	return &naiveLogsGrouper{valueExpression: valueExpression}, nil
}

var _ NewGrouperFunc[plog.Logs] = newNaiveLogsGrouper

func TestLogsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("matches naive implementation", func(t *testing.T) {
		subject := "log.attributes[\"subject\"]"
		telemetrySettings := componenttest.NewNopTelemetrySettings()
		ctx := context.Background()
		srcLogs := generateLogs(t)

		naiveLogsGrouper, err := newNaiveLogsGrouper(subject, telemetrySettings)
		require.NoError(t, err)
		logsGrouper, err := NewLogsGrouper(subject, telemetrySettings)
		assert.NoError(t, err)

		wantGroups, err := naiveLogsGrouper.Group(ctx, srcLogs)
		require.NoError(t, err)
		haveGroups, err := logsGrouper.Group(ctx, srcLogs)
		assert.NoError(t, err)

		compareGroups := func(a, b Group[plog.Logs]) int {
			return strings.Compare(a.Subject, b.Subject)
		}
		slices.SortFunc(wantGroups, compareGroups)
		slices.SortFunc(haveGroups, compareGroups)
		assert.Equal(t, len(wantGroups), len(haveGroups))
		for i := range len(wantGroups) {
			assert.NoError(t, plogtest.CompareLogs(
				wantGroups[i].Data,
				haveGroups[i].Data,
			))
		}
	})
}
