// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Driver                         string          `mapstructure:"driver"`
	DataSource                     string          `mapstructure:"datasource"`
	Queries                        []Query         `mapstructure:"queries"`
	StorageID                      *component.ID   `mapstructure:"storage"`
	Telemetry                      TelemetryConfig `mapstructure:"telemetry"`
}

func (c Config) Validate() error {
	if c.Driver == "" {
		return errors.New("'driver' cannot be empty")
	}
	if c.DataSource == "" {
		return errors.New("'datasource' cannot be empty")
	}
	if len(c.Queries) == 0 {
		return errors.New("'queries' cannot be empty")
	}
	for _, query := range c.Queries {
		if err := query.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Query struct {
	SQL                string      `mapstructure:"sql"`
	Metrics            []MetricCfg `mapstructure:"metrics"`
	Logs               []LogsCfg   `mapstructure:"logs"`
	TrackingColumn     string      `mapstructure:"tracking_column"`
	TrackingStartValue string      `mapstructure:"tracking_start_value"`
}

func (q Query) Validate() error {
	var errs []error
	if q.SQL == "" {
		errs = append(errs, errors.New("'query.sql' cannot be empty"))
	}
	if len(q.Logs) == 0 && len(q.Metrics) == 0 {
		errs = append(errs, errors.New("at least one of 'query.logs' and 'query.metrics' must not be empty"))
	}
	for _, logs := range q.Logs {
		if err := logs.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, metric := range q.Metrics {
		if err := metric.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type LogsCfg struct {
	BodyColumn       string   `mapstructure:"body_column"`
	AttributeColumns []string `mapstructure:"attribute_columns"`
}

func (config LogsCfg) Validate() error {
	var errs []error
	if config.BodyColumn == "" {
		errs = append(errs, errors.New("'body_column' must not be empty"))
	}
	return errors.Join(errs...)
}

type MetricCfg struct {
	MetricName       string            `mapstructure:"metric_name"`
	ValueColumn      string            `mapstructure:"value_column"`
	AttributeColumns []string          `mapstructure:"attribute_columns"`
	Monotonic        bool              `mapstructure:"monotonic"`
	ValueType        MetricValueType   `mapstructure:"value_type"`
	DataType         MetricType        `mapstructure:"data_type"`
	Aggregation      MetricAggregation `mapstructure:"aggregation"`
	Unit             string            `mapstructure:"unit"`
	Description      string            `mapstructure:"description"`
	StaticAttributes map[string]string `mapstructure:"static_attributes"`
	StartTsColumn    string            `mapstructure:"start_ts_column"`
	TsColumn         string            `mapstructure:"ts_column"`
}

func (c MetricCfg) Validate() error {
	var errs []error
	if c.MetricName == "" {
		errs = append(errs, errors.New("'metric_name' cannot be empty"))
	}
	if c.ValueColumn == "" {
		errs = append(errs, errors.New("'value_column' cannot be empty"))
	}
	if err := c.ValueType.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.DataType.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Aggregation.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.DataType == MetricTypeGauge && c.Aggregation != "" {
		errs = append(errs, fmt.Errorf("aggregation=%s but data_type=%s does not support aggregation", c.Aggregation, c.DataType))
	}
	if errs != nil && c.MetricName != "" {
		errs = append(errs, fmt.Errorf("invalid metric config with metric_name '%s'", c.MetricName))
	}
	return errors.Join(errs...)
}

type MetricType string

const (
	MetricTypeUnspecified MetricType = ""
	MetricTypeGauge       MetricType = "gauge"
	MetricTypeSum         MetricType = "sum"
)

func (t MetricType) Validate() error {
	switch t {
	case MetricTypeUnspecified, MetricTypeGauge, MetricTypeSum:
		return nil
	}
	return fmt.Errorf("metric config has unsupported data_type: '%s'", t)
}

type MetricValueType string

const (
	MetricValueTypeUnspecified MetricValueType = ""
	MetricValueTypeInt         MetricValueType = "int"
	MetricValueTypeDouble      MetricValueType = "double"
)

func (t MetricValueType) Validate() error {
	switch t {
	case MetricValueTypeUnspecified, MetricValueTypeInt, MetricValueTypeDouble:
		return nil
	}
	return fmt.Errorf("metric config has unsupported value_type: '%s'", t)
}

type MetricAggregation string

const (
	MetricAggregationUnspecified MetricAggregation = ""
	MetricAggregationCumulative  MetricAggregation = "cumulative"
	MetricAggregationDelta       MetricAggregation = "delta"
)

func (a MetricAggregation) Validate() error {
	switch a {
	case MetricAggregationUnspecified, MetricAggregationCumulative, MetricAggregationDelta:
		return nil
	}
	return fmt.Errorf("metric config has unsupported aggregation: '%s'", a)
}

type TelemetryConfig struct {
	Logs TelemetryLogsConfig `mapstructure:"logs"`
}

type TelemetryLogsConfig struct {
	Query bool `mapstructure:"query"`
}
