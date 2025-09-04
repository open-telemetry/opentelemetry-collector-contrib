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

func generateLogs() plog.Logs {
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
	parser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[ottllog.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	constructSubject := func(resourceLogs plog.ResourceLogs, scopeLogs plog.ScopeLogs, logRecord plog.LogRecord) string {
		subjectAsAny, err := valueExpression.Eval(t.Context(), ottllog.NewTransformContext(
			logRecord,
			scopeLogs.Scope(),
			resourceLogs.Resource(),
			scopeLogs,
			resourceLogs,
		))
		require.NoError(t, err)
		return subjectAsAny.(string)
	}

	subjects := make(map[string]bool)
	for _, srcResourceLogs := range srcLogs.ResourceLogs().All() {
		for _, srcScopeLogs := range srcResourceLogs.ScopeLogs().All() {
			for _, srcLogRecord := range srcScopeLogs.LogRecords().All() {
				subjects[constructSubject(srcResourceLogs, srcScopeLogs, srcLogRecord)] = true
			}
		}
	}

	groups := make([]Group[plog.Logs], 0, len(subjects))
	for groupSubject := range subjects {
		destLogs := plog.NewLogs()
		srcLogs.CopyTo(destLogs)
		destLogs.ResourceLogs().RemoveIf(func(destResourceLogs plog.ResourceLogs) bool {
			destResourceLogs.ScopeLogs().RemoveIf(func(destScopeLogs plog.ScopeLogs) bool {
				destScopeLogs.LogRecords().RemoveIf(func(destLogRecord plog.LogRecord) bool {
					return constructSubject(destResourceLogs, destScopeLogs, destLogRecord) != groupSubject
				})
				return destScopeLogs.LogRecords().Len() == 0
			})
			return destResourceLogs.ScopeLogs().Len() == 0
		})
		groups = append(groups, Group[plog.Logs]{
			Subject: groupSubject,
			Data:    destLogs,
		})
	}
	return groups
}

func TestLogsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		subject := "log.attributes[\"subject\"]"
		srcLogs := generateLogs()

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
