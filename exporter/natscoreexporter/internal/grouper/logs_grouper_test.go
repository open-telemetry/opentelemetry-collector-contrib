// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func groupLogs(t *testing.T, subject string, srcLogs plog.Logs) ([]Group[plog.Logs], error) {
	parser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[ottllog.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	constructSubject := func(resourceLogs plog.ResourceLogs, scopeLogs plog.ScopeLogs, logRecord plog.LogRecord) (string, error) {
		subjectAsAny, err := valueExpression.Eval(t.Context(), ottllog.NewTransformContext(
			logRecord,
			scopeLogs.Scope(),
			resourceLogs.Resource(),
			scopeLogs,
			resourceLogs,
		))
		if err != nil {
			return "", err
		}

		subject, ok := subjectAsAny.(string)
		if !ok {
			return "", errors.New("subject is not a string")
		}
		return subject, nil
	}

	subjects := make(map[string]bool)
	var errs error
	for _, srcResourceLogs := range srcLogs.ResourceLogs().All() {
		for _, srcScopeLogs := range srcResourceLogs.ScopeLogs().All() {
			for _, srcLogRecord := range srcScopeLogs.LogRecords().All() {
				subject, err := constructSubject(srcResourceLogs, srcScopeLogs, srcLogRecord)
				if err == nil {
					subjects[subject] = true
				} else {
					errs = multierr.Append(errs, err)
				}
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
					subject, err := constructSubject(destResourceLogs, destScopeLogs, destLogRecord)
					return err != nil || subject != groupSubject
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
	return groups, errs
}

func TestLogsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		logsDir := "testdata/logs"

		entries, err := os.ReadDir(logsDir)
		require.NoError(t, err)

		for _, entry := range entries {
			t.Run(entry.Name(), func(t *testing.T) {
				testCaseDir := filepath.Join(logsDir, entry.Name())

				subjectAsBytes, err := os.ReadFile(filepath.Join(testCaseDir, "subject.txt"))
				require.NoError(t, err)
				subject := string(subjectAsBytes)

				srcLogs, err := golden.ReadLogs(filepath.Join(testCaseDir, "logs.yaml"))
				require.NoError(t, err)

				logsGrouper, err := NewLogsGrouper(subject, componenttest.NewNopTelemetrySettings())
				assert.NoError(t, err)

				haveGroups, haveErr := logsGrouper.Group(t.Context(), srcLogs)
				wantGroups, wantErr := groupLogs(t, subject, srcLogs)
				assert.ElementsMatch(t, multierr.Errors(haveErr), multierr.Errors(wantErr))

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
	})
}
