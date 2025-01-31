// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutiltest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestNewLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		expected := plog.NewLogs()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogs("", "", "")))
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts()))
	})

	t.Run("simple", func(t *testing.T) {
		expected := func() plog.Logs {
			ld := plog.NewLogs()
			r := ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeB") // resourceA.scopeB
			l := s.LogRecords().AppendEmpty()
			l.Body().SetStr("logC") // resourceA.scopeB.logC
			return ld
		}()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogs("A", "B", "C")))
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts(
			plogutiltest.Resource("A", plogutiltest.Scope("B", plogutiltest.LogRecord("C"))),
		)))
	})

	t.Run("two_resources", func(t *testing.T) {
		expected := func() plog.Logs {
			ld := plog.NewLogs()
			r := ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceA.scopeC
			l := s.LogRecords().AppendEmpty()
			l.Body().SetStr("logD") // resourceA.scopeC.logD
			r = ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceB") // resourceB
			s = r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceB.scopeC
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logD") // resourceB.scopeC.logD
			return ld
		}()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogs("AB", "C", "D")))
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts(
			plogutiltest.Resource("A", plogutiltest.Scope("C", plogutiltest.LogRecord("D"))),
			plogutiltest.Resource("B", plogutiltest.Scope("C", plogutiltest.LogRecord("D"))),
		)))
	})

	t.Run("two_scopes", func(t *testing.T) {
		expected := func() plog.Logs {
			ld := plog.NewLogs()
			r := ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeB") // resourceA.scopeB
			l := s.LogRecords().AppendEmpty()
			l.Body().SetStr("logD") // resourceA.scopeB.logD
			s = r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceA.scopeC
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logD") // resourceA.scopeC.logD
			return ld
		}()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogs("A", "BC", "D")))
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts(
			plogutiltest.Resource("A",
				plogutiltest.Scope("B", plogutiltest.LogRecord("D")),
				plogutiltest.Scope("C", plogutiltest.LogRecord("D")),
			),
		)))
	})

	t.Run("two_records", func(t *testing.T) {
		expected := func() plog.Logs {
			ld := plog.NewLogs()
			r := ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeB") // resourceA.scopeB
			l := s.LogRecords().AppendEmpty()
			l.Body().SetStr("logC") // resourceA.scopeB.logC
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logD") // resourceA.scopeB.logD
			return ld
		}()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogs("A", "B", "CD")))
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts(
			plogutiltest.Resource("A", plogutiltest.Scope("B", plogutiltest.LogRecord("C"), plogutiltest.LogRecord("D"))),
		)))
	})

	t.Run("asymmetrical_scopes", func(t *testing.T) {
		expected := func() plog.Logs {
			ld := plog.NewLogs()
			r := ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceA.scopeC
			l := s.LogRecords().AppendEmpty()
			l.Body().SetStr("logE") // resourceA.scopeC.logE
			s = r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeD") // resourceA.scopeD
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logE") // resourceA.scopeD.logE
			r = ld.ResourceLogs().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceB") // resourceB
			s = r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scopeD") // resourceB.scopeD
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logF") // resourceB.scopeD.logF
			l = s.LogRecords().AppendEmpty()
			l.Body().SetStr("logG") // resourceB.scopeD.logG
			return ld
		}()
		assert.NoError(t, plogtest.CompareLogs(expected, plogutiltest.NewLogsFromOpts(
			plogutiltest.Resource("A",
				plogutiltest.Scope("C", plogutiltest.LogRecord("E")),
				plogutiltest.Scope("D", plogutiltest.LogRecord("E")),
			),
			plogutiltest.Resource("B",
				plogutiltest.Scope("D", plogutiltest.LogRecord("F"), plogutiltest.LogRecord("G")),
			),
		)))
	})
}
