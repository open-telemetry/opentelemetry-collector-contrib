// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/multierr"
)

var (
	// errTooManyBatchers is returned when the MetadataCardinalityLimit has been reached.
	errTooManyBatchers = consumererror.NewPermanent(errors.New("too many batcher metadata-value combinations"))
	// errUnexpectedType is returned when the object in the map isn't the expected type
	errUnexpectedType = errors.New("unexpected type in map")
)

type MetadataConfig struct {
	*Config

	// MetadataKeys is a list of client.Metadata keys that will be
	// used to form distinct batchers.  If this setting is empty,
	// a single batcher instance will be used.  When this setting
	// is not empty, one batcher will be used per distinct
	// combination of values for the listed metadata keys.
	//
	// Empty value and unset metadata are treated as distinct cases.
	//
	// Entries are case-insensitive.  Duplicated entries will
	// trigger a validation error.
	MetadataKeys []string `mapstructure:"metadata_keys"`

	// MetadataCardinalityLimit indicates the maximum number of
	// batcher instances that will be created through a distinct
	// combination of MetadataKeys.
	MetadataCardinalityLimit uint32 `mapstructure:"metadata_cardinality_limit"`
}

var _ component.Config = (*MetadataConfig)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *MetadataConfig) Validate() error {
	uniq := map[string]bool{}
	for _, k := range cfg.MetadataKeys {
		l := strings.ToLower(k)
		if _, has := uniq[l]; has {
			return fmt.Errorf("duplicate entry in metadata_keys: %q (case-insensitive)", l)
		}
		uniq[l] = true
	}
	return nil
}

func createMetadataDefaultConfig() component.Config {
	return &MetadataConfig{
		Config: createDefaultConfig().(*Config),
	}
}

type metadataExporter struct {
	config   *MetadataConfig
	settings exporter.Settings
	scf      streamClientFactory
	host     component.Host

	metadataKeys []string
	exporters    sync.Map

	// Guards the size and the storing logic to ensure no more than limit items are stored.
	// If we are willing to allow "some" extra items than the limit this can be removed and size can be made atomic.
	lock sync.Mutex
	size int
}

var _ exp = (*metadataExporter)(nil)

func newMetadataExporter(cfg component.Config, set exporter.Settings, streamClientFactory streamClientFactory) (exp, error) {
	oCfg := cfg.(*MetadataConfig)
	// use lower-case, to be consistent with http/2 headers.
	mks := make([]string, len(oCfg.MetadataKeys))
	for i, k := range oCfg.MetadataKeys {
		mks[i] = strings.ToLower(k)
	}
	sort.Strings(mks)
	if len(mks) == 0 {
		return newExporter(cfg, set, streamClientFactory)
	}
	return &metadataExporter{
		config:       oCfg,
		settings:     set,
		scf:          streamClientFactory,
		metadataKeys: mks,
	}, nil
}

func (e *metadataExporter) getSettings() exporter.Settings {
	return e.settings
}

func (e *metadataExporter) getConfig() component.Config {
	return e.config
}

func (e *metadataExporter) helperOptions() []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(e.config.Config.TimeoutSettings),
		exporterhelper.WithRetry(e.config.Config.RetryConfig),
		exporterhelper.WithQueue(e.config.Config.QueueSettings),
		exporterhelper.WithStart(e.start),
		exporterhelper.WithShutdown(e.shutdown),
	}
}

func (e *metadataExporter) start(ctx context.Context, host component.Host) (err error) {
	e.host = host
	return nil
}

func (e *metadataExporter) shutdown(ctx context.Context) error {
	var err error
	e.exporters.Range(func(key any, value any) bool {
		be, ok := value.(exp)
		if !ok {
			err = multierr.Append(err, fmt.Errorf("%w: %T", errUnexpectedType, value))
			return true
		}
		err = multierr.Append(err, be.shutdown(ctx))
		return true
	})
	return err
}

func (e *metadataExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	s := getSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s)
	if err != nil {
		return err
	}
	return be.pushTraces(ctx, td)
}

func (e *metadataExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	s := getSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s)
	if err != nil {
		return err
	}

	return be.(exp).pushMetrics(ctx, md)
}

func (e *metadataExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	s := getSet(ctx, e.metadataKeys)

	be, err := e.getOrCreateExporter(ctx, s)
	if err != nil {
		return err
	}

	return be.(exp).pushLogs(ctx, ld)
}

func (e *metadataExporter) getOrCreateExporter(ctx context.Context, s attribute.Set) (exp, error) {
	v, ok := e.exporters.Load(s)
	if !ok {
		e.lock.Lock()
		if e.config.MetadataCardinalityLimit != 0 && e.size >= int(e.config.MetadataCardinalityLimit) {
			e.lock.Unlock()
			return nil, errTooManyBatchers
		}

		newExp, err := newExporter(e.config, e.settings, e.scf)
		if err != nil {
			return nil, fmt.Errorf("failed to create exporter: %w", err)
		}

		var loaded bool
		v, loaded = e.exporters.LoadOrStore(s, newExp)
		if !loaded {
			// Start the goroutine only if we added the object to the map, otherwise is already started.
			err = newExp.start(ctx, e.host)
			if err != nil {
				e.exporters.Delete(s)
				return nil, fmt.Errorf("failed to start exporter: %w", err)
			}
			e.size++
		}
		e.lock.Unlock()
	}
	val, ok := v.(exp)
	if !ok {
		return nil, fmt.Errorf("%w: %T", errUnexpectedType, v)
	}
	return val, nil
}

func getSet(ctx context.Context, keys []string) attribute.Set {
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
	return attribute.NewSet(attrs...)
}
