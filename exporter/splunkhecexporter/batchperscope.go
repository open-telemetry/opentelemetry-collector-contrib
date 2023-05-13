// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

// perScopeBatcher is a consumer.Logs that rebatches logs by a type found in the scope name: profiling or regular logs.
type perScopeBatcher struct {
	logsEnabled      bool
	profilingEnabled bool
	next             consumer.Logs
}

func (rb *perScopeBatcher) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (rb *perScopeBatcher) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	var profilingFound bool
	var otherLogsFound bool

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rs := logs.ResourceLogs().At(i)
		for j := 0; j < rs.ScopeLogs().Len(); j++ {
			if isProfilingData(rs.ScopeLogs().At(j)) {
				profilingFound = true
			} else {
				otherLogsFound = true
			}
		}
		if profilingFound && otherLogsFound {
			break
		}
	}

	// if we don't have both types of logs, just call next if enabled
	if !profilingFound || !otherLogsFound {
		if rb.logsEnabled && otherLogsFound {
			return rb.next.ConsumeLogs(ctx, logs)
		}
		if rb.profilingEnabled && profilingFound {
			return rb.next.ConsumeLogs(ctx, logs)
		}
		return nil
	}

	profilingLogs := plog.NewLogs()
	otherLogs := plog.NewLogs()

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rs := logs.ResourceLogs().At(i)
		profilingFound = false
		otherLogsFound = false
		for j := 0; j < rs.ScopeLogs().Len(); j++ {
			sl := rs.ScopeLogs().At(j)
			if isProfilingData(sl) {
				profilingFound = true
			} else {
				otherLogsFound = true
			}
		}
		switch {
		case profilingFound && otherLogsFound:
			if rb.profilingEnabled {
				copyResourceLogs(rs, profilingLogs.ResourceLogs().AppendEmpty(), true)
			}
			if rb.logsEnabled {
				copyResourceLogs(rs, otherLogs.ResourceLogs().AppendEmpty(), false)
			}
		case profilingFound && rb.profilingEnabled:
			rs.CopyTo(profilingLogs.ResourceLogs().AppendEmpty())
		case otherLogsFound && rb.logsEnabled:
			rs.CopyTo(otherLogs.ResourceLogs().AppendEmpty())
		}
	}

	var err error
	if rb.logsEnabled {
		err = multierr.Append(err, rb.next.ConsumeLogs(ctx, otherLogs))
	}
	if rb.profilingEnabled {
		err = multierr.Append(err, rb.next.ConsumeLogs(ctx, profilingLogs))
	}
	return err
}

func copyResourceLogs(src plog.ResourceLogs, dest plog.ResourceLogs, isProfiling bool) {
	src.Resource().CopyTo(dest.Resource())
	for j := 0; j < src.ScopeLogs().Len(); j++ {
		sl := src.ScopeLogs().At(j)
		if isProfilingData(sl) == isProfiling {
			sl.CopyTo(dest.ScopeLogs().AppendEmpty())
		}
	}
}
