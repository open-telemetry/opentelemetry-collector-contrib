// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type groupingFileExporter struct {
	conf          *Config
	logger        *zap.Logger
	marshaller    *marshaller
	pathPrefix    string
	pathSuffix    string
	attribute     string
	maxOpenFiles  int
	newFileWriter func(path string) (*fileWriter, error)

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
	for pathSegment, rSpansSlice := range groups {
		traces := ptrace.NewTraces()
		for _, rSpans := range rSpansSlice {
			rSpans.CopyTo(traces.ResourceSpans().AppendEmpty())
		}

		buf, err := e.marshaller.marshalTraces(traces)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, pathSegment, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		return consumererror.NewPermanent(errs)
	}

	return nil
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
	for pathSegment, rMetricsSlice := range groups {
		metrics := pmetric.NewMetrics()
		for _, rMetrics := range rMetricsSlice {
			rMetrics.CopyTo(metrics.ResourceMetrics().AppendEmpty())
		}

		buf, err := e.marshaller.marshalMetrics(metrics)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, pathSegment, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		return consumererror.NewPermanent(errs)
	}

	return nil
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
	for pathSegment, rLogsSlice := range groups {
		logs := plog.NewLogs()
		for _, rlogs := range rLogsSlice {
			rlogs.CopyTo(logs.ResourceLogs().AppendEmpty())
		}

		buf, err := e.marshaller.marshalLogs(logs)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, pathSegment, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		return consumererror.NewPermanent(errs)
	}

	return nil
}

func (e *groupingFileExporter) consumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	if pd.ResourceProfiles().Len() == 0 {
		return nil
	}

	groups := make(map[string][]pprofile.ResourceProfiles)

	for i := 0; i < pd.ResourceProfiles().Len(); i++ {
		rProfiles := pd.ResourceProfiles().At(i)
		group(e, groups, rProfiles.Resource(), rProfiles)
	}

	var errs error
	for pathSegment, rProfilesSlice := range groups {
		profiles := pprofile.NewProfiles()
		for _, rProfiles := range rProfilesSlice {
			rProfiles.CopyTo(profiles.ResourceProfiles().AppendEmpty())
		}

		buf, err := e.marshaller.marshalProfiles(profiles)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		err = e.write(ctx, pathSegment, buf)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		return consumererror.NewPermanent(errs)
	}

	return nil
}

func (e *groupingFileExporter) write(_ context.Context, pathSegment string, buf []byte) error {
	writer, err := e.getWriter(pathSegment)
	if err != nil {
		return err
	}

	err = writer.export(buf)
	if err != nil {
		return err
	}

	return nil
}

func (e *groupingFileExporter) getWriter(pathSegment string) (*fileWriter, error) {
	fullPath := e.fullPath(pathSegment)

	e.mutex.Lock()
	defer e.mutex.Unlock()

	writer, ok := e.writers.Get(fullPath)
	if ok {
		return writer, nil
	}

	perm := os.FileMode(0o755)
	if e.conf.directoryPermissionsParsed != 0 {
		perm = os.FileMode(e.conf.directoryPermissionsParsed)
	}
	err := os.MkdirAll(path.Dir(fullPath), perm)
	if err != nil {
		return nil, err
	}

	writer, err = e.newFileWriter(fullPath)
	if err != nil {
		return nil, err
	}

	e.writers.Add(fullPath, writer)

	writer.start()

	return writer, nil
}

func cleanPathPrefix(pathPrefix string) string {
	cleaned := path.Clean(pathPrefix)
	if strings.HasSuffix(pathPrefix, "/") && !strings.HasSuffix(cleaned, "/") {
		return cleaned + "/"
	}

	return cleaned
}

func (e *groupingFileExporter) fullPath(pathSegment string) string {
	if strings.HasPrefix(pathSegment, "./") {
		pathSegment = pathSegment[1:]
	} else if strings.HasPrefix(pathSegment, "../") {
		pathSegment = pathSegment[2:]
	}

	fullPath := path.Clean(e.pathPrefix + pathSegment + e.pathSuffix)
	if strings.HasPrefix(fullPath, e.pathPrefix) {
		return fullPath
	}

	// avoid path traversal vulnerability
	return path.Join(e.pathPrefix, path.Join("/", pathSegment+e.pathSuffix))
}

func (e *groupingFileExporter) onEvict(_ string, writer *fileWriter) {
	err := writer.shutdown()
	if err != nil {
		e.logger.Warn("Failed to close file", zap.Error(err), zap.String("path", writer.path))
	}
}

func group[T any](e *groupingFileExporter, groups map[string][]T, resource pcommon.Resource, resourceEntries T) {
	var pathSegment string
	v, ok := resource.Attributes().Get(e.attribute)
	if ok {
		if v.Type() == pcommon.ValueTypeStr {
			pathSegment = v.Str()
		} else {
			ok = false
		}
	}

	if !ok {
		e.logger.Debug(fmt.Sprintf("Resource does not contain %s attribute, dropping it", e.attribute))
		return
	}

	groups[pathSegment] = append(groups[pathSegment], resourceEntries)
}

// Start initializes and starts the exporter.
func (e *groupingFileExporter) Start(_ context.Context, host component.Host) error {
	var err error
	e.marshaller, err = newMarshaller(e.conf, host)
	if err != nil {
		return err
	}
	export := buildExportFunc(e.conf)

	pathParts := strings.Split(e.conf.Path, "*")

	e.pathPrefix = cleanPathPrefix(pathParts[0])
	e.attribute = e.conf.GroupBy.ResourceAttribute
	e.pathSuffix = pathParts[1]
	e.maxOpenFiles = e.conf.GroupBy.MaxOpenFiles
	e.newFileWriter = func(path string) (*fileWriter, error) {
		return newFileWriter(path, e.conf.Append, e.conf.Rotation, e.conf.FlushInterval, export)
	}

	writers, err := simplelru.NewLRU(e.conf.GroupBy.MaxOpenFiles, e.onEvict)
	if err != nil {
		return err
	}

	e.writers = writers

	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops flushes and closes all underlying writers.
func (e *groupingFileExporter) Shutdown(context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.writers == nil {
		return nil
	}

	e.writers.Purge()
	e.writers = nil

	return nil
}
