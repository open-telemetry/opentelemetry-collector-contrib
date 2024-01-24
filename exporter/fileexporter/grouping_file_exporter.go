// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"errors"
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
	discardAtribute            bool
	discardIfAttributeNotFound bool
	defaultSubPath             string
	maxOpenFiles               int
	newFileWriter              func(path string) (*fileWriter, error)

	mutex   sync.Mutex
	writers *simplelru.LRU[string, *fileWriter]
}

func (e *groupingFileExporter) consumeTraces(ctx context.Context, td ptrace.Traces) error {
	groups := make(map[string][]ptrace.ResourceSpans)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rSpans := td.ResourceSpans().At(i)
		group(e, groups, rSpans.Resource(), rSpans)
	}

	var errs error
	for subPath, rSpansSlice := range groups {
		traces := ptrace.NewTraces()
		for _, rSpans := range rSpansSlice {
			rSpans.CopyTo(traces.ResourceSpans().AppendEmpty())
		}

		buf, err := e.marshaller.marshalTraces(traces)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		err = e.write(ctx, subPath, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (e *groupingFileExporter) consumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	groups := make(map[string][]pmetric.ResourceMetrics)

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rMetrics := md.ResourceMetrics().At(i)
		group(e, groups, rMetrics.Resource(), rMetrics)
	}

	var errs error
	for subPath, rMetricsSlice := range groups {
		metrics := pmetric.NewMetrics()
		for _, rMetrics := range rMetricsSlice {
			rMetrics.CopyTo(metrics.ResourceMetrics().AppendEmpty())
		}

		buf, err := e.marshaller.marshalMetrics(metrics)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		err = e.write(ctx, subPath, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (e *groupingFileExporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	groups := make(map[string][]plog.ResourceLogs)

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rLogs := ld.ResourceLogs().At(i)
		group(e, groups, rLogs.Resource(), rLogs)
	}

	var errs error
	for subPath, rLogsSlice := range groups {
		logs := plog.NewLogs()
		for _, rlogs := range rLogsSlice {
			rlogs.CopyTo(logs.ResourceLogs().AppendEmpty())
		}

		buf, err := e.marshaller.marshalLogs(logs)
		if err != nil {
			errs = errors.Join(errs, err)
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
	var err error
	if !ok {
		writer, err = e.newFileWriter(fullPath)
		if err != nil {
			return nil, err
		}

		e.writers.Add(fullPath, writer)

		err = writer.start(ctx)
		if err != nil {
			return nil, err
		}
	}

	return writer, nil
}

func (e *groupingFileExporter) onEnvict(fullPath string, writer *fileWriter) {
	writer.shutdown() // TODO: log error?
}

func group[T any](e *groupingFileExporter, groups map[string][]T, resource pcommon.Resource, resourceEntries T) {
	var subPath string
	v, ok := resource.Attributes().Get(e.attribute)
	if ok && v.Type() == pcommon.ValueTypeStr {
		subPath = v.Str()
	} else if e.discardIfAttributeNotFound {
		return
	} else {
		subPath = e.defaultSubPath
	}

	groups[subPath] = append(groups[subPath], resourceEntries)
}

// Start starts the flush timer if set. TODO
func (e *groupingFileExporter) Start(ctx context.Context, _ component.Host) error {
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops the flush ticker if set. TODO
func (e *groupingFileExporter) Shutdown(ctx context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.writers.Purge()

	return nil
}
