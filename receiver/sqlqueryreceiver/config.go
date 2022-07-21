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

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Driver                                  string  `mapstructure:"driver"`
	DataSource                              string  `mapstructure:"datasource"`
	Queries                                 []Query `mapstructure:"queries"`
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
	SQL     string      `mapstructure:"sql"`
	Metrics []MetricCfg `mapstructure:"metrics"`
}

func (q Query) Validate() error {
	var errs error
	if q.SQL == "" {
		errs = multierr.Append(errs, errors.New("'query.sql' cannot be empty"))
	}
	if len(q.Metrics) == 0 {
		errs = multierr.Append(errs, errors.New("'query.metrics' cannot be empty"))
	}
	for _, metric := range q.Metrics {
		if err := metric.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}

type MetricCfg struct {
	MetricName       string            `mapstructure:"metric_name"`
	ValueColumn      string            `mapstructure:"value_column"`
	AttributeColumns []string          `mapstructure:"attribute_columns"`
	Monotonic        bool              `mapstructure:"monotonic"`
	ValueType        MetricValueType   `mapstructure:"value_type"`
	DataType         MetricDataType    `mapstructure:"data_type"`
	Aggregation      MetricAggregation `mapstructure:"aggregation"`
	Unit             string            `mapstructure:"unit"`
	Description      string            `mapstructure:"description"`
}

func (c MetricCfg) Validate() error {
	var errs error
	if c.MetricName == "" {
		errs = multierr.Append(errs, errors.New("'metric_name' cannot be empty"))
	}
	if c.ValueColumn == "" {
		errs = multierr.Append(errs, errors.New("'value_column' cannot be empty"))
	}
	if err := c.ValueType.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.DataType.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.Aggregation.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if c.DataType == MetricDataTypeGauge && c.Aggregation != "" {
		errs = multierr.Append(errs, fmt.Errorf("aggregation=%s but data_type=%s does not support aggregation", c.Aggregation, c.DataType))
	}
	if errs != nil && c.MetricName != "" {
		errs = multierr.Append(fmt.Errorf("invalid metric config with metric_name '%s'", c.MetricName), errs)
	}
	return errs
}

type MetricDataType string

const (
	MetricDataTypeUnspecified MetricDataType = ""
	MetricDataTypeGauge       MetricDataType = "gauge"
	MetricDataTypeSum         MetricDataType = "sum"
)

func (t MetricDataType) Validate() error {
	switch t {
	case MetricDataTypeUnspecified, MetricDataTypeGauge, MetricDataTypeSum:
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

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: 10 * time.Second,
		},
	}
}
