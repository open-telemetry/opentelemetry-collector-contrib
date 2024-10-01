// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	arrowPkg "github.com/apache/arrow/go/v16/arrow"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/multierr"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

var (
	// errTooManyExporters is returned when the MetadataCardinalityLimit has been reached.
	errTooManyExporters = consumererror.NewPermanent(errors.New("too many exporter metadata-value combinations"))
)

type metadataExporter struct {
	config   *Config
	settings exporter.Settings
	scf      streamClientFactory
	host     component.Host

	metadataKeys []string
	exporters    sync.Map
	netReporter  *netstats.NetworkReporter

	userAgent string

	// Guards the size and the storing logic to ensure no more than limit items are stored.
	// If we are willing to allow "some" extra items than the limit this can be removed and size can be made atomic.
	lock sync.Mutex
	size int
}

var _ exp = (*metadataExporter)(nil)

func newMetadataExporter(cfg component.Config, set exporter.Settings, streamClientFactory streamClientFactory) (exp, error) {
	oCfg := cfg.(*Config)
	netReporter, err := netstats.NewExporterNetworkReporter(set)
	if err != nil {
		return nil, err
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	if !oCfg.Arrow.Disabled {
		// Ignoring an error because Validate() was called.
		_ = zstd.SetEncoderConfig(oCfg.Arrow.Zstd)

		userAgent += fmt.Sprintf(" ApacheArrow/%s (NumStreams/%d)", arrowPkg.PkgVersion, oCfg.Arrow.NumStreams)
	}
	// use lower-case, to be consistent with http/2 headers.
	mks := make([]string, len(oCfg.MetadataKeys))
	for i, k := range oCfg.MetadataKeys {
		mks[i] = strings.ToLower(k)
	}
	sort.Strings(mks)
	if len(mks) == 0 {
		return newExporter(cfg, set, streamClientFactory, userAgent, netReporter)
	}
	return &metadataExporter{
		config:       oCfg,
		settings:     set,
		scf:          streamClientFactory,
		metadataKeys: mks,
		userAgent:    userAgent,
		netReporter:  netReporter,
	}, nil
}

func (e *metadataExporter) getSettings() exporter.Settings {
	return e.settings
}

func (e *metadataExporter) getConfig() component.Config {
	return e.config
}

func (e *metadataExporter) start(_ context.Context, host component.Host) (err error) {
	e.host = host
	return nil
}

func (e *metadataExporter) shutdown(ctx context.Context) error {
	var err error
	e.exporters.Range(func(_ any, value any) bool {
		be := value.(exp)
		err = multierr.Append(err, be.shutdown(ctx))
		return true
	})
	return err
}

func (e *metadataExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	s, mdata := e.getAttrSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s, mdata)
	if err != nil {
		return err
	}
	return be.pushTraces(ctx, td)
}

func (e *metadataExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	s, mdata := e.getAttrSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s, mdata)
	if err != nil {
		return err
	}

	return be.pushMetrics(ctx, md)
}

func (e *metadataExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	s, mdata := e.getAttrSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s, mdata)
	if err != nil {
		return err
	}

	return be.pushLogs(ctx, ld)
}

func (e *metadataExporter) getOrCreateExporter(ctx context.Context, s attribute.Set, md metadata.MD) (exp, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.config.MetadataCardinalityLimit != 0 && e.size >= int(e.config.MetadataCardinalityLimit) {
		return nil, errTooManyExporters
	}

	v, ok := e.exporters.Load(s)
	if ok {
		return v.(exp), nil
	}

	newExp, err := newExporter(e.config, e.settings, e.scf, e.userAgent, e.netReporter)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	var loaded bool
	v, loaded = e.exporters.LoadOrStore(s, newExp)
	if !loaded {
		// set metadata keys for base exporter to add them to the outgoing context.
		newExp.(*baseExporter).setMetadata(md)

		// Start the goroutine only if we added the object to the map, otherwise is already started.
		err = newExp.start(ctx, e.host)
		if err != nil {
			e.exporters.Delete(s)
			return nil, fmt.Errorf("failed to start exporter: %w", err)
		}

		e.size++
	}

	return v.(exp), nil
}

// getAttrSet is code taken from the core collector's batchprocessor multibatch logic.
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.107.0/processor/batchprocessor/batch_processor.go#L298
func (e *metadataExporter) getAttrSet(ctx context.Context, keys []string) (attribute.Set, metadata.MD) {
	// Get each metadata key value, form the corresponding
	// attribute set for use as a map lookup key.
	info := client.FromContext(ctx)
	md := map[string][]string{}
	var attrs []attribute.KeyValue
	for _, k := range keys {
		// Lookup the value in the incoming metadata, copy it
		// into the outgoing metadata, and create a unique
		// value for the attributeSet.
		vs := info.Metadata.Get(k)
		md[k] = vs
		if len(vs) == 1 {
			attrs = append(attrs, attribute.String(k, vs[0]))
		} else {
			attrs = append(attrs, attribute.StringSlice(k, vs))
		}
	}
	return attribute.NewSet(attrs...), metadata.MD(md)
}
