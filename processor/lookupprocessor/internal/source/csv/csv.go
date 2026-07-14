// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package csv provides a CSV file-based lookup source. It supports headered and
// headerless files, returning a single column or the whole row as a map.
package csv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/csv"

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

const sourceType = "csv"

// Config is the configuration for the CSV lookup source.
type Config struct {
	lookupsource.FileSourceConfig `mapstructure:",squash"`

	// HasHeader indicates whether the first row is a header of column names.
	// Default: true. When false, columns are referenced by 0-based index.
	HasHeader bool `mapstructure:"has_header"`

	// Delimiter is the field delimiter. Default: ",". Must be a single character.
	Delimiter string `mapstructure:"delimiter"`

	// KeyColumn selects the lookup-key column by header name (requires has_header).
	KeyColumn string `mapstructure:"key_column"`
	// KeyColumnIndex selects the lookup-key column by 0-based index.
	KeyColumnIndex *int `mapstructure:"key_column_index"`

	// ValueColumn selects a single value column by header name (requires
	// has_header), making lookups return that column as a scalar.
	ValueColumn string `mapstructure:"value_column"`
	// ValueColumnIndex selects a single value column by 0-based index.
	ValueColumnIndex *int `mapstructure:"value_column_index"`
}

// Validate implements lookupsource.SourceConfig.
func (c *Config) Validate() error {
	if err := c.FileSourceConfig.Validate(); err != nil {
		return err
	}
	if len([]rune(c.Delimiter)) != 1 {
		return fmt.Errorf("delimiter must be a single character, got %q", c.Delimiter)
	}

	hasKeyName := c.KeyColumn != ""
	hasKeyIndex := c.KeyColumnIndex != nil
	switch {
	case hasKeyName && hasKeyIndex:
		return errors.New("only one of key_column or key_column_index may be set")
	case !hasKeyName && !hasKeyIndex:
		return errors.New("one of key_column or key_column_index is required")
	case hasKeyName && !c.HasHeader:
		return errors.New("key_column (by name) requires has_header: true; use key_column_index for a headerless CSV")
	case hasKeyIndex && *c.KeyColumnIndex < 0:
		return errors.New("key_column_index must not be negative")
	}

	hasValueName := c.ValueColumn != ""
	hasValueIndex := c.ValueColumnIndex != nil
	switch {
	case hasValueName && hasValueIndex:
		return errors.New("only one of value_column or value_column_index may be set")
	case hasValueName && !c.HasHeader:
		return errors.New("value_column (by name) requires has_header: true; use value_column_index for a headerless CSV")
	case hasValueIndex && *c.ValueColumnIndex < 0:
		return errors.New("value_column_index must not be negative")
	}

	return nil
}

// NewFactory creates a factory for the CSV source.
func NewFactory() lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		sourceType,
		createDefaultConfig,
		createSource,
	)
}

func createDefaultConfig() lookupsource.SourceConfig {
	return &Config{
		HasHeader: true,
		Delimiter: ",",
	}
}

func createSource(
	_ context.Context,
	settings lookupsource.CreateSettings,
	cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	csvCfg := cfg.(*Config)

	reloadMetrics, err := lookupsource.NewReloadMetrics(settings.TelemetrySettings, metadata.ScopeName)
	if err != nil {
		return nil, err
	}

	fl := lookupsource.NewFileLookup(lookupsource.FileLookupSettings{
		Path:           csvCfg.Path,
		ReloadInterval: csvCfg.ReloadInterval,
		Parse:          makeParse(csvCfg),
		Logger:         settings.TelemetrySettings.Logger,
		OnReload:       reloadMetrics.Record,
	})

	return lookupsource.NewSource(
		fl.Lookup,
		func() string { return sourceType },
		fl.Start,
		fl.Shutdown,
	), nil
}

// makeParse returns a ParseFunc for cfg. The header is re-read on every parse,
// so a reload still works if columns are reordered.
func makeParse(cfg *Config) lookupsource.ParseFunc {
	comma := ','
	if cfg.Delimiter != "" {
		comma, _ = utf8.DecodeRuneInString(cfg.Delimiter)
	}
	valueIsSet := cfg.ValueColumn != "" || cfg.ValueColumnIndex != nil

	return func(content []byte) (map[string]any, error) {
		reader := csv.NewReader(bytes.NewReader(content))
		reader.Comma = comma
		// Allow rows with different field counts. The bounds checks below skip
		// rows that are too short.
		reader.FieldsPerRecord = -1

		records, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return map[string]any{}, nil
		}

		var header []string
		dataRows := records
		if cfg.HasHeader {
			header = records[0]
			dataRows = records[1:]
		}

		keyIdx, err := resolveColumn("key", cfg.KeyColumn, cfg.KeyColumnIndex, header)
		if err != nil {
			return nil, err
		}
		valueIdx := -1
		if valueIsSet {
			if valueIdx, err = resolveColumn("value", cfg.ValueColumn, cfg.ValueColumnIndex, header); err != nil {
				return nil, err
			}
		}

		table := make(map[string]any, len(dataRows))
		for _, row := range dataRows {
			if keyIdx >= len(row) {
				continue
			}
			key := row[keyIdx]

			if valueIsSet {
				if valueIdx >= len(row) {
					continue
				}
				table[key] = row[valueIdx]
				continue
			}

			rowMap := make(map[string]any, len(row))
			for i, field := range row {
				if cfg.HasHeader && i < len(header) {
					rowMap[header[i]] = field
				} else {
					rowMap[strconv.Itoa(i)] = field
				}
			}
			table[key] = rowMap
		}
		return table, nil
	}
}

func resolveColumn(what, name string, index *int, header []string) (int, error) {
	if index != nil {
		return *index, nil
	}
	if idx := slices.Index(header, name); idx >= 0 {
		return idx, nil
	}
	return 0, fmt.Errorf("%s column %q not found in CSV header", what, name)
}
