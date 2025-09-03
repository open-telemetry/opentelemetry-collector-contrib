// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func generateLogs(t *testing.T) plog.Logs {
	t.Helper()

	logs := plog.NewLogs()
	for range 10 {
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resourceLogs.Resource().Attributes().PutStr("id", uuid.NewString())
		for range 10 {
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			scopeLogs.Scope().Attributes().PutStr("id", uuid.NewString())
			for range 10 {
				logRecord := scopeLogs.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("id", uuid.NewString())
				logRecord.Attributes().PutStr("subject", strconv.Itoa(rand.IntN(10)))
			}
		}
	}
	return logs
}

func groupLogs(t *testing.T, subject string, srcLogs plog.Logs) []Group[plog.Logs] {
	t.Helper()

	parser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[ottllog.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

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

				subjectAsAny, err := valueExpression.Eval(t.Context(), transformContext)
				require.NoError(t, err)
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
	return groups
}

func TestLogsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		subject := "log.attributes[\"subject\"]"
		srcLogs := generateLogs(t)

		logsGrouper, err := NewLogsGrouper(subject, componenttest.NewNopTelemetrySettings())
		assert.NoError(t, err)
		haveGroups, err := logsGrouper.Group(t.Context(), srcLogs)
		assert.NoError(t, err)

		wantGroups := groupLogs(t, subject, srcLogs)

		compareGroups := func(a, b Group[plog.Logs]) int {
			return strings.Compare(a.Subject, b.Subject)
		}
		slices.SortFunc(wantGroups, compareGroups)
		slices.SortFunc(haveGroups, compareGroups)

		assert.Len(t, wantGroups, len(haveGroups))
		for i := range len(wantGroups) {
			assert.NoError(t, plogtest.CompareLogs(
				wantGroups[i].Data,
				haveGroups[i].Data,
			))
		}
	})
}
