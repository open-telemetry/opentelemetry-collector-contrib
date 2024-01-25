// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"errors"
	"os"
	"path"
	"sync"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type groupingFileExporter struct {
	marshaller                 *marshaller
	basePath                   string
	attribute                  string
	deleteAtribute             bool
	discardIfAttributeNotFound bool
	defaultSubPath             string
	maxOpenFiles               int
	createDirs                 bool
	newFileWriter              func(path string) (*fileWriter, error)

	mutex   sync.Mutex
	writers *simplelru.LRU[string, *fileWriter]
}

func (e *groupingFileExporter) consumeTraces(ctx context.Context, td ptrace.Traces) error {
	if td.ResourceSpans().Len() == 0 {
		return nil
	}

	groups := make(map[string][]ptrace.ResourceSpans)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rSpans := td.ResourceSpans().At(i)
		group(e, groups, rSpans.Resource(), rSpans)
	}

	var errs error
	for subPath, rSpansSlice := range groups {
		traces := ptrace.NewTraces()
		for _, rSpans := range rSpansSlice {
			rSpans.MoveTo(traces.ResourceSpans().AppendEmpty())
		}

		buf, err := e.marshaller.marshalTraces(traces)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, subPath, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (e *groupingFileExporter) consumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if md.ResourceMetrics().Len() == 0 {
		return nil
	}

	groups := make(map[string][]pmetric.ResourceMetrics)

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rMetrics := md.ResourceMetrics().At(i)
		group(e, groups, rMetrics.Resource(), rMetrics)
	}

	var errs error
	for subPath, rMetricsSlice := range groups {
		metrics := pmetric.NewMetrics()
		for _, rMetrics := range rMetricsSlice {
			rMetrics.MoveTo(metrics.ResourceMetrics().AppendEmpty())
		}

		buf, err := e.marshaller.marshalMetrics(metrics)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, subPath, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (e *groupingFileExporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	if ld.ResourceLogs().Len() == 0 {
		return nil
	}

	groups := make(map[string][]plog.ResourceLogs)

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rLogs := ld.ResourceLogs().At(i)
		group(e, groups, rLogs.Resource(), rLogs)
	}

	var errs error
	for subPath, rLogsSlice := range groups {
		logs := plog.NewLogs()
		for _, rlogs := range rLogsSlice {
			rlogs.MoveTo(logs.ResourceLogs().AppendEmpty())
		}

		buf, err := e.marshaller.marshalLogs(logs)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, subPath, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (e *groupingFileExporter) write(ctx context.Context, subPath string, buf []byte) error {
	writer, err := e.getWriter(ctx, subPath)

	if err != nil {
		return err
	}

	err = writer.export(buf)
	if err != nil {
		return err
	}

	return nil
}

func (e *groupingFileExporter) getWriter(ctx context.Context, subPath string) (*fileWriter, error) {
	// avoid path traversal vulnerability
	subPath = path.Join("/", subPath)

	fullPath := path.Join(e.basePath, subPath)

	e.mutex.Lock()
	defer e.mutex.Unlock()

	writer, ok := e.writers.Get(fullPath)
	if ok {
		return writer, nil
	}

	var err error
	if e.createDirs {
		err = os.MkdirAll(path.Dir(fullPath), 0755)
		if err != nil {
			return nil, err
		}
	}

	writer, err = e.newFileWriter(fullPath)
	if err != nil {
		return nil, err
	}

	e.writers.Add(fullPath, writer)

	err = writer.start(ctx)
	if err != nil {
		return nil, err
	}

	return writer, nil
}

func (e *groupingFileExporter) onEnvict(_ string, writer *fileWriter) {
	_ = writer.shutdown() // TODO: should we log this error?
}

func group[T any](e *groupingFileExporter, groups map[string][]T, resource pcommon.Resource, resourceEntries T) {
	var subPath string
	v, ok := resource.Attributes().Get(e.attribute)
	if ok {
		if v.Type() == pcommon.ValueTypeStr {
			subPath = v.Str()
		} else {
			ok = false
		}

		if e.deleteAtribute {
			resource.Attributes().Remove(e.attribute)
		}
	}

	if !ok {
		if e.discardIfAttributeNotFound {
			return
		}

		subPath = e.defaultSubPath
	}

	groups[subPath] = append(groups[subPath], resourceEntries)
}

// Start is a noop.
func (e *groupingFileExporter) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops flushes and closes all underlying writers.
func (e *groupingFileExporter) Shutdown(context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.writers.Purge()

	return nil
}
